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
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
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

	godotenv.Load()

	total_threads := *threads

	// Initialize termbox
	// TermInit()

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
		Keys:    bson.D{{Key: "title", Value: -1}, {Key: "url", Value: -1}},
		Options: options.Index().SetUnique(true),
	}

	_, err4 := GetDBInstance().Collection("doujin").Indexes().CreateOne(context.TODO(), indexModel)
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

	t := make(chan bool, 1)

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

				if ok, _ := DoujinExists(context.TODO(), r.Title, r.URL); ok {
					log.Printf("Title %s already exists. - Cache", r.Title)
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

						/* for _, value := range r.Pages {
							if value.URL == NOT_FOUND {
								log.Println("NOT_FOUND")
								return
							}
						}

						if len(r.Pages) != r.TotalPages {
							log.Println("NOT_EQUAL_PAGES")
							return
						} */

						/* for _, pp := range r.page_map {
							log.Println(pp)
						} */
						log.Println(r.ID, r.Title, r.URL)

						callback := func(sessCtx mongo.SessionContext) (interface{}, error) {

							err := InsertToDoujinCollection(&r, sessCtx)

							if err != nil {
								return nil, err
							}
							var pages []Page

							for _, p := range r.page_map {
								pages = append(pages, p)
							}
							err, id_arr := InsertManyDoujinPages(r.ID, &pages, sessCtx)

							if err != nil {
								return nil, err
							}
							err2 := UpdateDoujin(r.ID, sessCtx, bson.D{{Key: "$set", Value: bson.D{{Key: "pages", Value: id_arr}}}})

							if err2 != nil {
								return nil, err2
							}
							return nil, nil
						}

						session, err := client.StartSession()
						check(err)

						defer session.EndSession(context.TODO())

						_, err3 := session.WithTransaction(context.TODO(), callback)

						check(err3)

						log.Printf("Added %s\n", r.Title)
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

func SaveToJSON(a any, file string) {
	bytes, err := json.MarshalIndent(a, "", "    ")

	check(err)

	f, err := os.Create(file)
	check(err)
	defer f.Close()

	_, errf := f.Write(bytes)

	check(errf)
}
