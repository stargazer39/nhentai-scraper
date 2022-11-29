package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Doujin struct {
	ID         primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	Title      string               `json:"title" bson:"title"`
	URL        string               `json:"url" bson:"url"`
	Thumb      string               `json:"thumb" bson:"thumb"`
	TotalPages int                  `json:"total" bson:"total"`
	Pages      []primitive.ObjectID `json:"pages" bson:"pages"`
	Tags       map[string][]string  `json:"tags" bson:"tags"`
	page_map   map[int]Page
	err        bool
	mutex      *sync.Mutex
	client     *HTTPClient
}

type Page struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Number    int                `json:"number" bson:"number,omitempty"`
	URL       string             `json:"url" bson:"url,omitempty"`
	Cached    bool               `json:"cached" bson:"cached"`
	CachedURL string             `json:"cached_url" bson:"cached_url"`
	DoujinID  primitive.ObjectID `json:"d_id,omitempty" bson:"d_id,omitempty"`
}

type TaskProgress struct {
	Done     int
	ID       int
	Finished bool
	Final    Doujin
	Total    int
}

func NewDoujin(title string, url string, thumb string, client *HTTPClient) *Doujin {
	return &Doujin{
		ID:       primitive.NewObjectID(),
		Title:    title,
		URL:      url,
		Thumb:    thumb,
		mutex:    &sync.Mutex{},
		client:   client,
		page_map: make(map[int]Page),
		Tags:     make(map[string][]string),
	}
}

func (doujin *Doujin) ResolvePage(base_url *url.URL, page int) error {
	page++

	page_url, err := GetDoujinPageURL(base_url, doujin.URL, page)

	if err != nil {
		check(err)
	}

	page_path := page_url.String()

	resp, err := doujin.client.Get(page_path, http.StatusOK)

	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		return err
	}

	// Get page image URL
	image := doc.Find("#image-container img").First().AttrOr("src", NOT_FOUND)

	if image == NOT_FOUND {
		return fmt.Errorf("image src not found on title %s", doujin.Title)
	}

	if doujin.ID == (primitive.ObjectID{}) {
		return fmt.Errorf("doujin id is empty")
	}

	page_content := Page{
		Number:   page,
		URL:      image,
		ID:       primitive.NewObjectID(),
		DoujinID: doujin.ID,
	}

	doujin.mutex.Lock()

	doujin.page_map[page] = page_content

	doujin.mutex.Unlock()
	return nil
}

func PageResolveTask(page_path string, page int) {

}

func (doujin *Doujin) ResolveDoujinDetails(base_url *url.URL) error {
	page_url, err := GetDoujinPageURL(base_url, doujin.URL, -1)

	if err != nil {
		check(err)
	}

	page_path := page_url.String()

	resp, err := doujin.client.Get(page_path, http.StatusOK)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		return err
	}

	// Resolve total pages

	total_s := doc.Find("#info > div:nth-child(4)").Text()

	r, _ := regexp.Compile(`^[^\d]*(\d+)`)

	total_s = r.FindString(total_s)
	total, err := strconv.Atoi(total_s)

	if err != nil {
		return err
	}

	doujin.TotalPages = total

	if total == 0 {
		return fmt.Errorf("error. total 0")
	}

	// Resolve tags

	tags_elem := doc.Find("#tags > .tag-container")

	tags := make(map[string][]string)

	tags_elem.Each(func(i int, s *goquery.Selection) {
		key := strings.ToLower(strings.TrimSpace(s.Contents().First().Text()))

		s.Find(".tag").Each(func(i int, s *goquery.Selection) {
			tags[key] = append(tags[key], strings.ToLower(s.Text()))
		})
	})

	doujin.Tags = tags

	// TODO - Resolve last updated
	return nil
}
