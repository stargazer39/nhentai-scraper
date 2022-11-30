package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DoujinV2 struct {
	Gallery struct {
		ID     int `json:"id,omitempty" bson:"id,omitempty"`
		Images struct {
			Cover struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"cover,omitempty" bson:"cover,omitempty"`
			Pages []struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"pages,omitempty"`
			Thumbnail struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"thumbnail,omitempty" bson:"thumbnail,omitempty"`
		} `json:"images,omitempty" bson:"images,omitempty"`
		MediaID   string `json:"media_id,omitempty" bson:"media_id,omitempty"`
		NumPages  int    `json:"num_pages,omitempty" bson:"num_pages,omitempty"`
		Scanlator string `json:"scanlator,omitempty" bson:"scanlator,omitempty"`
		Tags      []struct {
			CreatedAt time.Time   `json:"created_at,omitempty" bson:"created_at,omitempty"`
			DeletedAt interface{} `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
			ID        int         `json:"id,omitempty" bson:"id,omitempty"`
			Name      string      `json:"name,omitempty" bson:"name,omitempty"`
			NhID      string      `json:"nh_id,omitempty" bson:"nh_id,omitempty"`
			Pivot     struct {
				BookID int `json:"book_id,omitempty" bson:"book_id,omitempty"`
				TagID  int `json:"tag_id,omitempty" bson:"tag_id,omitempty"`
			} `json:"pivot,omitempty" bson:"pivot,omitempty"`
			Type      string    `json:"type,omitempty" bson:"type,omitempty"`
			UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
			URL       string    `json:"url,omitempty" bson:"url,omitempty"`
		} `json:"tags,omitempty" bson:"tags,omitempty"`
		Title struct {
			English  string `json:"english,omitempty" bson:"english,omitempty"`
			Japanese string `json:"japanese,omitempty" bson:"japanese,omitempty"`
			Pretty   string `json:"pretty,omitempty" bson:"pretty,omitempty"`
		} `json:"title,omitempty" bson:"title,omitempty"`
		UploadDate int `json:"upload_date,omitempty" bson:"upload_date,omitempty"`
	} `json:"gallery,omitempty" bson:"gallery,omitempty"`
	MediaURL  string `json:"media_url,omitempty" bson:"media_url,omitempty"`
	StartPage int    `json:"start_page,omitempty" bson:"start_page,omitempty"`
}

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
