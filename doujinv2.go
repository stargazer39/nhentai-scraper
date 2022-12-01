package main

import (
	"time"

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
