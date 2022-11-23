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
	"github.com/remeh/sizedwaitgroup"
)

type Doujin struct {
	Title      string              `json:"title" bson:"title"`
	Url        string              `json:"url" bson:"url"`
	Thumb      string              `json:"thumb" bson:"thumb"`
	TotalPages int                 `json:"total" bson:"total"`
	Pages      []Page              `json:"pages" bson:"pages"`
	Tags       map[string][]string `json:"tags" bson:"tags"`
	done_pages int
	mutex      sync.Mutex
	err        bool
}

type Page struct {
	Number int    `json:"number" bson:"number,omitempty"`
	URL    string `json:"url" bson:"url,omitempty"`
}

type TaskProgress struct {
	Done     int
	ID       int
	Finished bool
	Final    Doujin
	Total    int
}

func (doujin *Doujin) ResolvePages(base_url *url.URL, wg *sizedwaitgroup.SizedWaitGroup, progress chan TaskProgress, task_id int) error {
	for i := 1; i <= doujin.TotalPages; i++ {
		page_url, err := GetDoujinPageURL(base_url, doujin.Url, i)

		if err != nil {
			check(err)
		}

		page_path := page_url.String()

		wg.Add()
		go func(page_path string, page int) error {
			defer func() {
				doujin.mutex.Lock()
				doujin.done_pages++

				finished := doujin.done_pages == doujin.TotalPages

				tp := TaskProgress{
					Done:     doujin.done_pages,
					ID:       task_id,
					Finished: finished,
					Total:    doujin.TotalPages,
				}

				if finished {
					tp.Final = *doujin
				}

				progress <- tp
				doujin.mutex.Unlock()
				wg.Done()
			}()

			resp, err := http.Get(page_path)

			if err != nil {
				return err
			}

			doc, err := goquery.NewDocumentFromReader(resp.Body)

			if err != nil {
				return err
			}

			image := doc.Find("#image-container img").First().AttrOr("src", "not found")

			doujin.mutex.Lock()
			doujin.Pages = append(doujin.Pages, Page{
				Number: page,
				URL:    image,
			})

			// log.Println(image)
			doujin.mutex.Unlock()
			return nil
		}(page_path, i)
	}
	return nil
}

func (doujin *Doujin) ResolveDoujinDetails(base_url *url.URL) error {
	page_url, err := GetDoujinPageURL(base_url, doujin.Url, -1)
	if err != nil {
		check(err)
	}

	page_path := page_url.String()

	resp, err := http.Get(page_path)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)

	if err != nil {
		return err
	}

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

	// TODO - Resolve tags and other shit

	tags_elem := doc.Find("#tags > .tag-container")

	tags := make(map[string][]string)

	tags_elem.Each(func(i int, s *goquery.Selection) {
		key := strings.ToLower(strings.TrimSpace(s.Contents().First().Text()))

		s.Find(".tag").Each(func(i int, s *goquery.Selection) {
			tags[key] = append(tags[key], strings.ToLower(s.Text()))
		})
	})

	doujin.Tags = tags
	return nil
}
