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

	u, _ := url.Parse(HOME)

	http_client := NewHTTPClient(
		&http.Client{
			Timeout: 240 * time.Second,
		},
	)

	wg2 := sizedwaitgroup.New(total_threads)
	total := 0

	progress_page := NewProgressWatcher("Pages")
	progress_page.SetTotal(float32(*stop - *start))

	t := make(chan bool, 12)

	// GEt all in the collection
	opts := options.Find().SetProjection(bson.D{{Key: "url", Value: 1}})
	cur, err := doujin_collection.Find(context.TODO(), bson.M{}, opts)

	check(err)

	var doujin_arr []Doujin
	doujin_map := make(map[string]int)
	err3 := cur.All(context.TODO(), &doujin_arr)

	check(err3)

	for _, d := range doujin_arr {
		doujin_map[d.Url] = 1
	}

	for i := *start; i <= *stop; i++ {
		new_url := setURLQuery(u, "page", fmt.Sprint(i))
		page_url := new_url.String()

		// log.Println("done1")
		t <- true
		// log.Println("done2")
		go func(page_url string) {
			defer func() {
				<-t
			}()

			doujins, err := GetGallery(page_url, http_client)

			if err != nil {
				log.Printf("Getting %s failed. %v\n", page_url, err)
				return
			}

			for _, r := range doujins {

				if _, ok := doujin_map[r.Url]; ok {
					log.Printf("Title %s already exists. - Cache", r.Title)
					continue
				}

				count, err := doujin_collection.CountDocuments(context.TODO(), bson.D{{Key: "title", Value: r.Title}, {Key: "url", Value: r.Url}})

				check(err)

				if count > 0 {
					log.Printf("Title %s already exists", r.Title)
					continue
				}

				err2 := r.ResolveDoujinDetails(u)
				// log.Println("resolve")
				if err2 != nil {
					log.Println(err2)
					r.err = true
					log.Printf("Skipping %s due to error %v", r.Title, err)
					continue
				} else {
					resolve_done := make(chan bool)

					go func(r Doujin) {
						co := 0

						for r.TotalPages != co {
							// Prog
							<-resolve_done
							co++
						}

						for _, value := range r.Pages {
							if value.URL == NOT_FOUND {
								log.Println("NOT_FOUND")
								return
							}
						}

						if len(r.Pages) != r.TotalPages {
							log.Println("NOT_EQUAL_PAGES")
							return
						}

						_, err := doujin_collection.InsertOne(context.TODO(), r)

						log.Printf("Added %s\n", r.Title)
						check(err)
					}(r)

					total += r.TotalPages
					for pg := 0; pg < r.TotalPages; pg++ {
						wg2.Add()
						go func(pg int, r Doujin) {
							defer wg2.Done()

							// log.Println("resolve page")
							err2 := r.ResolvePage(u, pg)

							resolve_done <- true
							check(err2)
						}(pg, r)
					}
					// log.Println("Success")
				}

			}

		}(page_url)
		progress_page.SetCurrentFunc(func(current float32) float32 {
			return current + 1
		})
	}

	wg2.Wait()
	log.Println("Starting to resolve doujin details")
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
		str, _ := s.Find("a").Attr("href")
		name := s.Find(".caption").Text()
		img, _ := s.Find("img").Attr("data-src")

		d := Doujin{
			Title:  name,
			Url:    str,
			Thumb:  img,
			Pages:  make(map[int]Page),
			mutex:  &sync.Mutex{},
			client: client,
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
