package main

import (
	"fmt"
	"net/url"
)

func GetDoujinPageURL(base_url *url.URL, doujin_url string, page_number int) (*url.URL, error) {
	page_path, err := url.JoinPath(base_url.Path, doujin_url, fmt.Sprint(page_number))
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
