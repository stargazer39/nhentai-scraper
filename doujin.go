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

type DoujinV2 struct {
	ID      primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Gallery struct {
		ID     int `json:"id,omitempty" bson:"id,omitempty"`
		Images struct {
			Cover struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"cover,omitempty" bson:"cover,omitempty"`
			Pages []struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"pages,omitempty"`
			Thumbnail struct {
				H int    `json:"h,omitempty" bson:"h,omitempty"`
				T string `json:"t,omitempty" bson:"t,omitempty"`
				W int    `json:"w,omitempty" bson:"w,omitempty"`
			} `json:"thumbnail,omitempty" bson:"thumbnail,omitempty"`
		} `json:"images,omitempty" bson:"images,omitempty"`
		MediaID   string `json:"media_id,omitempty" bson:"media_id,omitempty"`
		NumPages  int    `json:"num_pages,omitempty" bson:"num_pages,omitempty"`
		Scanlator string `json:"scanlator,omitempty" bson:"scanlator,omitempty"`
		Tags      []struct {
			CreatedAt time.Time   `json:"created_at,omitempty" bson:"created_at,omitempty"`
			DeletedAt interface{} `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
			ID        int         `json:"id,omitempty" bson:"id,omitempty"`
			Name      string      `json:"name,omitempty" bson:"name,omitempty"`
			NhID      string      `json:"nh_id,omitempty" bson:"nh_id,omitempty"`
			Pivot     struct {
				BookID int `json:"book_id,omitempty" bson:"book_id,omitempty"`
				TagID  int `json:"tag_id,omitempty" bson:"tag_id,omitempty"`
			} `json:"pivot,omitempty" bson:"pivot,omitempty"`
			Type      string    `json:"type,omitempty" bson:"type,omitempty"`
			UpdatedAt time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
			URL       string    `json:"url,omitempty" bson:"url,omitempty"`
		} `json:"tags,omitempty" bson:"tags,omitempty"`
		Title struct {
			English  string `json:"english,omitempty" bson:"english,omitempty"`
			Japanese string `json:"japanese,omitempty" bson:"japanese,omitempty"`
			Pretty   string `json:"pretty,omitempty" bson:"pretty,omitempty"`
		} `json:"title,omitempty" bson:"title,omitempty"`
		UploadDate int `json:"upload_date,omitempty" bson:"upload_date,omitempty"`
	} `json:"gallery,omitempty" bson:"gallery,omitempty"`
	MediaURL  string `json:"media_url,omitempty" bson:"media_url,omitempty"`
	StartPage int    `json:"start_page,omitempty" bson:"start_page,omitempty"`
}

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
