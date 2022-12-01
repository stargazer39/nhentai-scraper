package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go"
	"github.com/remeh/sizedwaitgroup"
	"github.com/robertkrimen/otto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var HTTP_CLIENT = NewHTTPClient(
	&http.Client{
		Timeout: 10 * time.Second,
	},
)

func main() {
	log.Println("Starting the scraper")
	start := flag.Int("start", 1, "Starting page")
	stop := flag.Int("stop", -1, "Stop page")
	threads := flag.Int("t", 12, "Threads")
	mode := flag.String("mode", "client", "Set to client mode")
	port := flag.String("p", ":4040", "Set Port")

	flag.Parse()
	client_uris := flag.Args()

	godotenv.Load()

	total_threads := *threads

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Mongo", os.Getenv("MONGO_URI"))
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))

	check(err)

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	SetDBInstance(client)

	// Initialize
	var (
		endpoint        = os.Getenv("MINIO_ENPOINT")
		accessKeyID     = os.Getenv("MINIO_ACCESS_KEY_ID")
		secretAccessKey = os.Getenv("MINIO_ENPOINT_SECRET_ACCESS_KEY")
		useSSL          = false
	)

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}

	SetMinioInstance(minioClient)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "gallery.id", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	indexModelPages := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	_, err4 := GetDBInstance().Collection("doujin").Indexes().CreateOne(context.TODO(), indexModel)
	if err4 != nil {
		check(err4)
	}

	_, err10 := GetDBInstance().Collection("pages").Indexes().CreateOne(context.TODO(), indexModelPages)
	if err10 != nil {
		check(err4)
	}

	HOME_URL, _ := url.Parse(HOME)

	log.Printf("Starting with %d threads\n", total_threads)
	wg := sizedwaitgroup.New(total_threads)

	page := *start

	jsvm := otto.New()
	var vm_mutex sync.Mutex
	output := []DoujinV2{}

	check(err)

	if *mode == "client" {
		// Start the client
		handler := NewDoujinHandler(HOME_URL, HTTP_CLIENT, &vm_mutex, jsvm)
		rpc.Register(handler)
		rpc.HandleHTTP()

		log.Printf("Listening RPC on %s\n", *port)
		err := http.ListenAndServe(*port, nil)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	var rpc_arr []*rpc.Client
	var rpc_mutex sync.Mutex
	curr_client := 0

	if *mode == "scraper" {
		for _, client := range client_uris {
			c, err := rpc.DialHTTP("tcp", client)
			if err != nil {
				log.Println("dialing:", err)
				continue
			}

			rpc_arr = append(rpc_arr, c)
		}

		if len(rpc_arr) == 0 {
			log.Panicln("No RPC clients are available")
		}

		for {
			wg.Add()
			go func(page int) {
				defer wg.Done()

				doujins, err := GetGallery(SetURLQuery(HOME_URL, "page", fmt.Sprint(page)).String(), HTTP_CLIENT)

				check(err)
				// log.Printf("Page %d \n", page)
				if len(doujins) == 0 {
					return
				}

				for _, doujin := range doujins {
					var reply string
					var this_client int

					rpc_mutex.Lock()
					this_client = curr_client

					curr_client++
					if curr_client == len(rpc_arr) {
						curr_client = 0
					}

					rpc_mutex.Unlock()

					err = rpc_arr[this_client].Call("DoujinHandler.ResolveDoujin", &doujin, &reply)
					if err != nil {
						log.Fatal("error:", err)
					}

					// log.Printf("reply %d %s ", this_client, reply)
				}
			}(page)

			if *stop == page {
				break
			}
			page++
			log.Printf("\n\n\nDone %d, Current Total %d \n\n\n", page, len(output))
		}

		wg.Wait()

		SaveToJSON(output, "result.json")
		log.Printf("\nSaved %d doujins. Total %d\n", len(output), len(output))
	}

	if *mode == "images" {
		for _, client := range client_uris {
			c, err := rpc.DialHTTP("tcp", client)
			if err != nil {
				log.Println("dialing:", err)
				continue
			}

			rpc_arr = append(rpc_arr, c)
		}

		if len(rpc_arr) == 0 {
			log.Panicln("No RPC clients are available")
		}

		// i, _ := primitive.ObjectIDFromHex("6387c227472b92ef6b30cb24")
		doujin, err := FindDoujin(context.TODO(), bson.D{})

		check(err)

		done := 0

		for _, d := range *doujin {
			wg.Add()
			go func(d DoujinV2) {
				defer wg.Done()

				for ii := 0; ii < d.GetTotalPages(); ii++ {
					// log.Println(d.GetPage(ii))
					rpc_mutex.Lock()
					var reply string
					this_client := curr_client

					curr_client++
					if curr_client == len(rpc_arr) {
						curr_client = 0
					}

					// log.Println("cr", curr_client, len(rpc_arr))

					page_page := d.GetPage(ii)

					err = rpc_arr[this_client].Call("DoujinHandler.CachePageURL", &page_page, &reply)
					rpc_mutex.Unlock()
					if err != nil {
						log.Fatal("error:", err)
					}
				}
			}(d)
			done++
			log.Printf("Done %d/%d\n", done, len(*doujin))
		}

	}
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
