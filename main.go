package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/remeh/sizedwaitgroup"
	"github.com/robertkrimen/otto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log.Println("Starting the scraper")
	start := flag.Int("start", 1, "Starting page")
	stop := flag.Int("stop", -1, "Stop page")
	threads := flag.Int("t", 12, "Threads")
	flag.Parse()

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

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "gallery.id", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	_, err4 := GetDBInstance().Collection("doujin").Indexes().CreateOne(context.TODO(), indexModel)
	if err4 != nil {
		check(err4)
	}

	home_url, _ := url.Parse(HOME)

	http_client := NewHTTPClient(
		&http.Client{
			Timeout: 2 * time.Second,
		},
	)

	log.Printf("Starting with %d threads\n", total_threads)
	wg := sizedwaitgroup.New(total_threads)

	progress_page := NewProgressWatcher("Pages")
	progress_page.SetTotal(float32(*stop - *start))

	page := *start
	find_n_data, err := regexp.Compile(`N\.reader\([\s\S]*?(}\);)`)
	jsvm := otto.New()
	var vm_mutex sync.Mutex
	output := []DoujinV2{}

	check(err)
	for {

		doujins, err := GetGallery(SetURLQuery(home_url, "page", fmt.Sprint(page)).String(), http_client)

		check(err)
		// log.Printf("Page %d \n", page)

		if len(doujins) == 0 {
			break
		}

		for _, doujin := range doujins {
			wg.Add()
			go func(p Doujin) {
				t1 := time.Now()

				defer wg.Done()
				// log.Println(p.Title)

				page_url, err := GetDoujinPageURL(home_url, p.URL, 1)
				// log.Println(page_url)
				if err != nil {
					check(err)
				}

				page_path := page_url.String()

				bytes, err := http_client.GetBytes(page_path, http.StatusOK)

				if err != nil {
					check(err)
				}

				if len(bytes) == 0 {
					log.Panicln("No match")
				}

				match := find_n_data.Find(bytes)

				if len(match) == 0 {
					log.Panicln("Length 0")
				}

				jstr := string(match)

				vm_mutex.Lock()

				result, err4 := jsvm.Run(`result = ` + jstr[9:len(jstr)-2])

				check(err4)

				json_bytes, err := result.MarshalJSON()

				check(err)

				var data DoujinV2

				err5 := json.Unmarshal(json_bytes, &data)
				check(err5)

				output = append(output, data)

				vm_mutex.Unlock()

				derr := InsertToDoujinCollection(&data, context.TODO())

				if IsCode(derr, 11000) {
					log.Printf("Dulicate %s \n", data.Gallery.Title.English)
					return
				}

				check(derr)
				du := time.Since(t1)

				log.Printf("Took %s\n", du.String())
				// log.Printf("Done %s\n", p.Title)
			}(doujin)
		}

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

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func GetGallery(page_url string, client *HTTPClient) ([]Doujin, error) {
	resp, err := client.Get(page_url, http.StatusOK)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		return nil, err
	}

	var doujin_arr []Doujin

	if doc.Find(".gallery").Length() == 0 {
		return nil, fmt.Errorf("no galleries in this page")
	}

	doc.Find(".gallery").Each(func(j int, s *goquery.Selection) {
		doujin_url, _ := s.Find("a").Attr("href")
		name := s.Find(".caption").Text()
		img, _ := s.Find("img").Attr("data-src")

		// Check name
		if len(strings.TrimSpace(name)) == 0 {
			log.Println("Name is Empty")
			return
		}

		// Check thumbnail
		if _, err := url.Parse(img); err != nil {
			log.Printf("Thumb is Empty\n%v", err)
			return
		}

		// Check url
		if _, err := url.Parse(doujin_url); err != nil {
			log.Printf("Thumb is Empty\n%v", err)
			return
		}

		d := *NewDoujin(name, doujin_url, img, client)

		doujin_arr = append(doujin_arr, d)
	})

	return doujin_arr, nil
}
