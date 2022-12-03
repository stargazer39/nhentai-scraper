package main

import (
	"context"
	"log"
	"net/http"
	"net/rpc"
	"net/url"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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
		wg:          sizedwaitgroup.New(1000),
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

	/* ok, err := PageExist(context.TODO(), page.Name)

	check(err)

	if ok {
		log.Println("Page exist")
		return nil
	} */

	rh.wg.Add()
	go func() {
		defer rh.wg.Done()
		resp, err := rh.http_client.Get(page.URL, http.StatusOK)

		if err != nil {
			log.Printf("Failed to add %s", page.URL)
			return
		}

		_, err = uploader.Upload(&s3manager.UploadInput{
			Bucket: bucket,
			Key:    aws.String(page.Name),
			Body:   resp.Body,
		})

		if err != nil {
			log.Printf("Failed to add %s", page.URL)
			return
		}
		/* file, err := os.Create(path.Join("/usr/rclone/sliit-drive/", page.Name))

		if err != nil {
			check(err)
		}

		_, err = io.Copy(file, resp.Body)

		if err != nil {
			log.Printf("Failed to add %s, %v", page.URL, err)
			return
		} */

		// if err := GetFTPInstance().Store(page.Name, resp.Body); err != nil {
		// 	check(err)
		// }
		/* _, err5 := GetMinioInstance().PutObject("nhentai", page.Name, io.NopCloser(resp.Body), resp.ContentLength, minio.PutObjectOptions{ContentType: resp.Header.Get("Content-type")})

		if err5 != nil {
			log.Fatalln(err5)
			return
		} */

		ierr := InsertToPageCollection(page, context.Background())

		if ierr != nil {
			log.Fatalln(ierr)
		}

		log.Println(page)
	}()

	return nil
}
