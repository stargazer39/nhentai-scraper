package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/robertkrimen/otto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Doujin struct {
	ID    primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Title string             `json:"title" bson:"title"`
	URL   string             `json:"url" bson:"url"`
	Thumb string             `json:"thumb" bson:"thumb"`
}

func NewDoujin(title string, url string, thumb string, client *HTTPClient) *Doujin {
	return &Doujin{
		ID:    primitive.NewObjectID(),
		Title: title,
		URL:   url,
		Thumb: thumb,
	}
}

func (doujin *Doujin) GetDoujinV2(home_url *url.URL, http_client *HTTPClient, vm_mutex *sync.Mutex, jsvm *otto.Otto) error {
	t1 := time.Now()
	page_url, err := GetDoujinPageURL(home_url, doujin.URL, 1)
	if err != nil {
		return err
	}

	page_path := page_url.String()

	bytes, err := http_client.GetBytes(page_path, http.StatusOK)

	if err != nil {
		return err
	}

	if len(bytes) == 0 {
		return fmt.Errorf("no match")
	}

	match := FIND_DATA_REGEX.Find(bytes)

	if len(match) == 0 {
		return fmt.Errorf("length 0")
	}

	jstr := string(match)

	vm_mutex.Lock()

	result, err4 := jsvm.Run(`result = ` + jstr[9:len(jstr)-2])

	if err4 != nil {
		return err4
	}

	json_bytes, err := result.MarshalJSON()

	if err != nil {
		return err
	}

	var data DoujinV2

	err5 := json.Unmarshal(json_bytes, &data)
	if err5 != nil {
		return err5
	}

	vm_mutex.Unlock()

	derr := InsertToDoujinCollection(&data, context.TODO())

	if IsCode(derr, 11000) {
		log.Printf("Dulicate %s \n", data.Gallery.Title.English)
		return nil
	}

	if derr != nil {
		return derr
	}
	du := time.Since(t1)

	log.Printf("Took %s\n", du.String())

	return nil
}
