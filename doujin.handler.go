package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/rpc"
	"net/url"
	"sync"

	"github.com/minio/minio-go"
	"github.com/remeh/sizedwaitgroup"
	"github.com/robertkrimen/otto"
)

type DoujinHandler struct {
	home_url    *url.URL
	http_client *HTTPClient
	vm_mutex    *sync.Mutex
	jsvm        *otto.Otto
	wg          sizedwaitgroup.SizedWaitGroup
}

func NewDoujinHandler(home_url *url.URL, http_client *HTTPClient, vm_mutex *sync.Mutex, jsvm *otto.Otto) *DoujinHandler {
	h := &DoujinHandler{
		home_url:    home_url,
		http_client: http_client,
		vm_mutex:    vm_mutex,
		jsvm:        jsvm,
		wg:          sizedwaitgroup.New(100),
	}
	err := rpc.Register(h)
	if err != nil {
		panic(err)
	}
	return h
}

func (rh *DoujinHandler) ResolveDoujin(d *Doujin, reply *string) error {
	if d == nil {
		log.Println("Doujin is nil")
		return nil
	}

	rh.wg.Add()
	go func() {
		defer rh.wg.Done()
		err := d.GetDoujinV2(rh.home_url, rh.http_client, rh.vm_mutex, rh.jsvm)

		if err != nil {
			log.Printf("Failed to add %s", d.Title)
		}
	}()

	s := "SUCCESS"
	reply = &s

	return nil
}

func (rh *DoujinHandler) CachePageURL(page *Page, reply *string) error {
	if page == nil {
		log.Println("Image is nil")
		return nil
	}

	ok, err := PageExist(context.TODO(), page.Name)

	check(err)

	if ok {
		log.Println("Page exist")
		return nil
	}

	rh.wg.Add()
	go func() {
		defer rh.wg.Done()
		resp, err := rh.http_client.Get(page.URL, http.StatusOK)

		if err != nil {
			log.Printf("Failed to add %s", page.URL)
			return
		}

		_, err5 := GetMinioInstance().PutObject("nhentai", page.Name, io.NopCloser(resp.Body), resp.ContentLength, minio.PutObjectOptions{ContentType: resp.Header.Get("Content-type")})

		if err5 != nil {
			log.Fatalln(err5)
			return
		}

		ierr := InsertToPageCollection(page, context.Background())

		if ierr != nil {
			log.Fatalln(ierr)
		}

		log.Println(page)
	}()

	return nil
}
