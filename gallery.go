package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
