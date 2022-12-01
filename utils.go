package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
)

func GetDoujinPageURL(base_url *url.URL, doujin_url string, page_number int) (*url.URL, error) {
	var page_path string
	var err error

	if page_number < 0 {
		page_path, err = url.JoinPath(base_url.Path, doujin_url)
	} else {
		page_path, err = url.JoinPath(base_url.Path, doujin_url, fmt.Sprint(page_number))
	}

	if err != nil {
		return nil, err
	}

	base_copy := *base_url
	base_copy.Path = page_path

	return &base_copy, nil
}

func GetURLFromTag(base_url *url.URL, tag string) (*url.URL, error) {
	page_path, err := url.JoinPath(base_url.Path, "tag", tag)
	if err != nil {
		return nil, err
	}

	base_copy := *base_url
	base_copy.Path = page_path

	return &base_copy, nil
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

func SetURLQuery(u *url.URL, query string, value string) *url.URL {
	q := u.Query()
	q.Set(query, value)

	new_url, _ := url.Parse(u.String())
	new_url.RawQuery = q.Encode()
	return new_url
}

func IsCode(err error, code int) bool {
	if we, ok := err.(mongo.WriteException); ok {
		for _, e := range we.WriteErrors {
			if e.Code == code {
				return true
			}
		}
	}

	return false
}
