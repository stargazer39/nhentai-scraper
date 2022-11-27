package main

import (
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
			time.Sleep(time.Millisecond * 250)
		}

		retry++

		resp, err := h.client.Get(url)

		if err != nil {
			continue
		}

		if resp.StatusCode != expect {
			continue
		}

		return resp, err
	}
}
