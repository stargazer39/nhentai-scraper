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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/joho/godotenv"
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

var s3Client *s3.S3
var bucket *string
var uploader *s3manager.Uploader

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

	// Connect to backblaze
	bucket = aws.String(os.Getenv("S3_BUCKET"))
	endpoint := aws.String(os.Getenv("S3_ENDPOINT"))
	keyId := os.Getenv("S3_KEY_ID")
	appKey := os.Getenv("S3_APP_KEY")
	region := aws.String(os.Getenv("S3_REGION"))

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(keyId, appKey, ""),
		Endpoint:         endpoint,
		Region:           region,
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession, err := session.NewSession(s3Config)

	check(err)

	s3Client = s3.New(newSession)
	uploader = s3manager.NewUploader(newSession)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "gallery.id", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	indexModel2 := mongo.IndexModel{
		Keys: bson.D{{Key: "gallery.num_pages", Value: -1}},
	}

	indexModelPages := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	indexModelPages2 := mongo.IndexModel{
		Keys: bson.D{{Key: "name", Value: -1}, {Key: "url", Value: -1}},
	}

	_, err4 := GetDBInstance().Collection("doujin").Indexes().CreateOne(context.TODO(), indexModel)
	if err4 != nil {
		check(err4)
	}

	_, err6 := GetDBInstance().Collection("doujin").Indexes().CreateOne(context.TODO(), indexModel2)
	if err6 != nil {
		check(err4)
	}

	_, err10 := GetDBInstance().Collection("pages").Indexes().CreateOne(context.TODO(), indexModelPages)
	if err10 != nil {
		check(err4)
	}

	_, err12 := GetDBInstance().Collection("pages").Indexes().CreateOne(context.TODO(), indexModelPages2)
	if err12 != nil {
		check(err12)
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
		log.Println("fetching existing")
		opt := options.Aggregate().SetBatchSize(1000)

		cur, err := GetDBInstance().Collection("doujin").Aggregate(context.TODO(), GET_INCOMPLETE_DOUJINS_WITH_PAGES, opt)

		check(err)

		/* var page_result []GetIncompleteDoujinsWithPages

		log.Println("query complete. objectifying.")
		err2 := cur.All(context.TODO(), &page_result) */

		// check(err2)
		done := 0

		// log.Println(len(page_result))

		log.Println("starting service")

		for cur.Next(context.TODO()) {
			var res GetIncompleteDoujinsWithPages

			if err := cur.Decode(&res); err != nil {
				log.Fatal(err)
			}

			//

			pages := res.Gallery.Images.Pages
			existing_pages := res.Pages

			// log.Printf("%d %d\n", len(pages), len(existing_pages))
			if len(pages) == len(existing_pages) {
				log.Panic("Query wrong")
			}

			for _, ep := range existing_pages {
				pages[ep.Page].skip = true
			}

			res.Gallery.Images.Pages = pages

			wg.Add()
			go func(d GetIncompleteDoujinsWithPages) {
				defer wg.Done()

				for ii := 0; ii < d.Gallery.GetTotalPages(); ii++ {
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

					if page_page == nil {
						rpc_mutex.Unlock()
						continue
					}

					/* if ok, _ := PageExist(context.TODO(), page_page.Name); ok {
						log.Printf("Exist %s %d \n", d.Gallery.Title.English, ii)
						rpc_mutex.Unlock()
						continue
					} */

					err = rpc_arr[this_client].Call("DoujinHandler.CachePageURL", &page_page, &reply)
					rpc_mutex.Unlock()
					if err != nil {
						log.Fatal("error:", err)
					}
				}
			}(res)
			done++
			log.Printf("Done %d\n", done)
		}

	}
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
