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
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/remeh/sizedwaitgroup"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log.Println("Starting the scraper")
	start := flag.Int("start", 1, "Starting page")
	stop := flag.Int("stop", 2, "Stop page")
	threads := flag.Int("t", 12, "Threads")
	flag.Parse()

	total_threads := *threads

	// Initialize termbox
	// TermInit()

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb+srv://stargazer-2:0wdyOv85cDtSoUwC@cluster0.y9yur.mongodb.net/?retryWrites=true&w=majority"))

	check(err)

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	db := client.Database("testing")
	doujin_collection := db.Collection("doujin")

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "title", Value: -1}, {Key: "url", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	_, err4 := doujin_collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err4 != nil {
		check(err4)
	}

	doujin_arr := []Doujin{}
	doujin_mutex := sync.Mutex{}

	u, _ := url.Parse(HOME)

	p1 := make(chan Progress)

	// Progress
	go func() {
		done := float32(0)

		for {
			p := <-p1

			if p.Reset {
				done = 0
				continue
			}

			done++

			// MoveToBeginning()

			log.Printf("%f/%f Progress", done, p.Total)
			if p.End {
				break
			}
		}
	}()

	wg := sizedwaitgroup.New(total_threads)
	http_client := http.Client{
		Timeout: 120 * time.Second,
	}

	for i := *start; i <= *stop; i++ {
		new_url := setURLQuery(u, "page", fmt.Sprint(i))
		page_url := new_url.String()

		wg.Add()
		go func(page_url string) {
			defer wg.Done()

			doujins, err := GetGallery(page_url, http_client)

			if err != nil {
				p1 <- Progress{
					Info:  fmt.Sprintf("Getting %s failed. %v\n", page_url, err),
					Total: float32(*stop),
				}
				return
			}

			doujin_mutex.Lock()

			for _, r := range doujins {
				count, err := doujin_collection.CountDocuments(context.TODO(), bson.D{{Key: "title", Value: r.Title}, {Key: "url", Value: r.Url}})

				check(err)

				if count > 0 {
					log.Printf("Title %s already exists", r.Title)
					continue
				}
				doujin_arr = append(doujin_arr, r)
			}

			// doujin_arr = append(doujin_arr, doujins...)
			doujin_mutex.Unlock()

			p1 <- Progress{
				Current: -1,
				Total:   float32(*stop),
			}
		}(page_url)
	}

	wg.Wait()

	log.Println("Starting to resolve doujin details")

	// MoveDown(1)

	p1 <- Progress{
		Reset: true,
	}

	wg2 := sizedwaitgroup.New(total_threads)

	total := len(doujin_arr)

	for i := range doujin_arr {
		err := doujin_arr[i].ResolveDoujinDetails(u)
		// log.Println("resolve")
		if err != nil {
			log.Println(err)
			doujin_arr[i].err = true
			log.Printf("Skipping %s due to error %v", doujin_arr[i].Title, err)
			return
		} else {
			resolve_done := make(chan bool)

			go func(i int) {
				co := 0

				for doujin_arr[i].TotalPages != co {
					<-resolve_done
					co++
				}

				for _, value := range doujin_arr[i].Pages {
					if value.URL == NOT_FOUND {
						log.Println("NOT_FOUND")
						return
					}
				}

				if len(doujin_arr[i].Pages) != doujin_arr[i].TotalPages {
					log.Println("NOT_EQUAL_PAGES")
					return
				}

				_, err := doujin_collection.InsertOne(context.TODO(), doujin_arr[i])

				check(err)

				p1 <- Progress{
					Total: float32(total),
				}
			}(i)

			for pg := 0; pg < doujin_arr[i].TotalPages; pg++ {
				wg2.Add()
				go func(pg int, i int) {
					defer wg2.Done()

					// log.Println("resolve page")
					err2 := doujin_arr[i].ResolvePage(u, pg)

					resolve_done <- true
					check(err2)
				}(pg, i)
			}

			// log.Println("Success")
		}
	}

	// log.Println("Resolve tasks were added. waiting them to be finished")
	wg2.Wait()

	log.Println("Saving to JSON")
	SaveToJSON(doujin_arr, "all.json")
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func setURLQuery(u *url.URL, query string, value string) *url.URL {
	q := u.Query()
	q.Set(query, value)

	new_url, _ := url.Parse(u.String())
	new_url.RawQuery = q.Encode()
	return new_url
}

func GetGallery(page_url string, client http.Client) ([]Doujin, error) {
	resp, err := client.Get(page_url)

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
		str, _ := s.Find("a").Attr("href")
		name := s.Find(".caption").Text()
		img, _ := s.Find("img").Attr("data-src")

		d := Doujin{
			Title: name,
			Url:   str,
			Thumb: img,
			Pages: make(map[int]Page),
		}

		// log.Printf("Title - %s\nURL - %s\nThumb - %s\n\n", name, str, img)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(str) == "" || strings.TrimSpace(img) == "" {
			return
		}
		// log.Println(img)

		doujin_arr = append(doujin_arr, d)
	})

	return doujin_arr, nil
}

func SaveToJSON(a any, file string) {
	bytes, err := json.MarshalIndent(a, "", "    ")

	check(err)

	f, err := os.Create(file)
	check(err)
	defer f.Close()

	_, errf := f.Write(bytes)

	check(errf)
}
