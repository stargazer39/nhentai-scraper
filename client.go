package main

import (
	"io"
	"log"
	"net/http"
	"time"
)

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient(c *http.Client) *HTTPClient {
	return &HTTPClient{
		client: c,
	}
}

func (h *HTTPClient) Get(url string, expect int) (*http.Response, error) {
	retry := 0

	for {
		if retry != 0 {
			time.Sleep(time.Second * 2)
		}

		retry++

		resp, err := h.client.Get(url)

		if err != nil {
			log.Println("Retry", err)
			continue
		}

		if resp.StatusCode != expect {
			log.Println("Retry", resp.Status)
			continue
		}

		return resp, err
	}
}

func (h *HTTPClient) GetBytes(url string, expect int) ([]byte, error) {
	for {
		resp, err := h.Get(url, expect)

		if err != nil {
			log.Println(err)
			continue
		}

		bytes, err := io.ReadAll(resp.Body)

		if err != nil {
			log.Println(err)
			continue
		}

		return bytes, err
	}
}
