package main

import (
	"log"
	"net/rpc"
	"net/url"
	"sync"

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

	// add logic to return users
	return nil
}
