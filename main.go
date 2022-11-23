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

type TasksWithErrors []func() error

func main() {
	log.Println("Starting the scraper")
	start := flag.Int("start", 1, "Starting page")
	stop := flag.Int("stop", 2, "Stop page")
	threads := flag.Int("t", 12, "Threads")
	flag.Parse()

	total_threads := *threads
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
	doujin := db.Collection("doujin")

	doujin_arr := []Doujin{}
	doujin_mutex := sync.Mutex{}

	u, _ := url.Parse(HOME)

	wg := sizedwaitgroup.New(total_threads)

	for i := *start; i <= *stop; i++ {
		new_url := setURLQuery(u, "page", fmt.Sprint(i))
		page_url := new_url.String()

		wg.Add()

		go func(page_url string) {
			doujins, err := GetGallery(page_url)

			if err != nil {
				log.Printf("Getting %s failed. %v\n", page_url, err)
				return
			}

			log.Printf("Got %s Total - %d\n", page_url, len(doujins))

			doujin_mutex.Lock()
			doujin_arr = append(doujin_arr, doujins...)
			doujin_mutex.Unlock()

			wg.Done()
		}(page_url)
	}

	wg.Wait()

	log.Println("Starting to resolve doujin details")
	wg = sizedwaitgroup.New(total_threads)

	for i := range doujin_arr {
		wg.Add()
		go func(index int) {
			err := doujin_arr[index].ResolveDoujinDetails(u)

			if err != nil {
				log.Println(err)
				doujin_arr[index].err = true
			}

			// log.Printf("%s is details done\n", doujin_arr[index].Title)
			wg.Done()
		}(i)
	}

	wg.Wait()

	log.Println("Starting to resolve doujin pages")

	wgn := sizedwaitgroup.New(total_threads)
	progress := make(chan TaskProgress)

	total := len(doujin_arr)

	// Monitor thread
	go func() {
		for total > 0 {
			task_progress := <-progress

			log.Printf("%d progress is %d/%d", task_progress.ID, task_progress.Done, task_progress.Total)
			if task_progress.Finished {
				log.Printf("%d is done", task_progress.ID)
				defer func() {
					total--
					log.Println(total)
				}()

				final := task_progress.Final
				check_fail := false

				for _, p := range final.Pages {
					if p.URL == "not found" {
						check_fail = true
						break
					}
				}

				if len(final.Pages) != final.TotalPages {
					check_fail = true
				}

				if !check_fail {
					_, err2 := doujin.InsertOne(context.TODO(), final)
					log.Printf("%s added", final.Title)
					check(err2)
				} else {
					log.Printf("%s was not added. sorry.", final.Title)
				}
			}
		}
	}()

	for index := range doujin_arr {
		if doujin_arr[index].err {
			log.Printf("Skipping %s due to error ", doujin_arr[index].Title)
			return
		}

		count, err := doujin.CountDocuments(context.TODO(), bson.D{{Key: "title", Value: doujin_arr[index].Title}, {Key: "url", Value: doujin_arr[index].Url}})

		check(err)

		if count > 0 {
			log.Printf("Title %s already exists", doujin_arr[index].Title)
			continue
		}

		err2 := doujin_arr[index].ResolvePages(u, &wgn, progress, index)
		log.Println("1111111111111111111111111")
		check(err2)
	}

	log.Println("Resolve tasks were added. waiting them to be finished")
	wgn.Wait()

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

func GetGallery(page_url string) ([]Doujin, error) {
	resp, err := http.Get(page_url)

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
		}

		log.Printf("Title - %s\nURL - %s\nThumb - %s\n\n", name, str, img)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(str) == "" || strings.TrimSpace(img) == "" {
			return
		}
		log.Println(img)

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
