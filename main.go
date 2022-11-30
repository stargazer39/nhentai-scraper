package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log.Println("Starting the scraper")
	start := flag.Int("start", 1, "Starting page")
	stop := flag.Int("stop", 2, "Stop page")
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

	home_url, _ := url.Parse(HOME)

	http_client := NewHTTPClient(
		&http.Client{
			Timeout: 240 * time.Second,
		},
	)

	wg := sizedwaitgroup.New(total_threads)

	progress_page := NewProgressWatcher("Pages")
	progress_page.SetTotal(float32(*stop - *start))

	page := 1
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
				defer wg.Done()
				// log.Println(p.Title)

				page_url, err := GetDoujinPageURL(home_url, doujin.URL, 1)
				// log.Println(page_url)
				if err != nil {
					check(err)
				}

				page_path := page_url.String()

				resp, err := http_client.Get(page_path, http.StatusOK)

				if err != nil {
					check(err)
				}

				bytes, berr := io.ReadAll(resp.Body)

				check(berr)

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
				// log.Printf("Done %s\n", p.Title)
			}(doujin)
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
