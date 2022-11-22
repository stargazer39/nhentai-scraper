package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/remeh/sizedwaitgroup"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Doujin struct {
	Title      string   `json:"title" bson:"title,omitempty"`
	Url        string   `json:"url" bson:"url,omitempty"`
	Thumb      string   `json:"thumb" bson:"thumb,omitempty"`
	TotalPages int      `json:"total" bson:"total,omitempty"`
	Pages      []string `json:"pages" bson:"pages,omitempty"`
	done_pages int
	mutex      sync.Mutex
}

type TasksWithErrors []func() error

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	log.Println("Starting the scraper")
	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))

	check(err)

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	// db := client.Database("testing")
	// doujin := db.Collection("doujin")

	doujin_arr := []Doujin{}
	doujin_mutex := sync.Mutex{}

	u, _ := url.Parse(HOME)

	wg := sizedwaitgroup.New(1000)

	for i := 1; i <= 10; i++ {
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
	wg = sizedwaitgroup.New(1000)

	for i := range doujin_arr {
		wg.Add()
		go func(index int) {
			doujin_arr[index].ResolveDoujinDetails(u)
			wg.Done()
		}(i)
	}

	wg.Wait()

	log.Println("Starting to resolve doujin pages")
	wg = sizedwaitgroup.New(1000)

	for index := range doujin_arr {
		wg.Add()
		go func(index int) {
			err := doujin_arr[index].ResolvePages(u)
			check(err)
			wg.Done()
		}(index)
	}

	wg.Wait()
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

	doc.Find(".gallery").Each(func(j int, s *goquery.Selection) {
		str, _ := s.Find("a").Attr("href")
		name := s.Find(".caption").Text()
		img, _ := s.Find("img").Attr("src")

		d := Doujin{
			Title: name,
			Url:   str,
			Thumb: img,
		}

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

func (doujin *Doujin) ResolveDoujinDetails(base_url *url.URL) error {
	d_url_path, err := url.JoinPath(base_url.Path, doujin.Url)

	if err != nil {
		return err
	}

	d_url, err := url.Parse(base_url.String())

	if err != nil {
		return err
	}

	d_url.Path = d_url_path + "1"

	page_path := d_url.String()

	resp, err := http.Get(page_path)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		return err
	}

	total_s := doc.Find(".num-pages").First().Text()

	total, err := strconv.Atoi(total_s)

	if err != nil {
		return err
	}

	doujin.TotalPages = total
	log.Printf("%s Total - %d", doujin.Url, total)

	// TODO - Resolve tags and other shit

	return nil
}

func (doujin *Doujin) ResolvePages(base_url *url.URL) error {
	for i := 1; i <= doujin.TotalPages; i++ {
		d_url_path, _ := url.JoinPath(base_url.Path, doujin.Url)
		d_url, _ := url.Parse(base_url.String())
		d_url.Path = d_url_path + fmt.Sprint(i)

		page_path := d_url.String()

		resp, err := http.Get(page_path)

		if err != nil {
			return err
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)

		if err != nil {
			return err
		}

		image := doc.Find("#image-container > a > img").AttrOr("src", "not found")
		doujin.Pages = append(doujin.Pages, image)
	}

	return nil
}
