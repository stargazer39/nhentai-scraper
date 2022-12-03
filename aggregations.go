package main

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var GET_DUJIN_WITH_INCOMPLETE_CACHE = bson.A{
	bson.D{
		{Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: "$gid"},
				{Key: "pages", Value: bson.D{{Key: "$count", Value: bson.D{}}}},
			},
		},
	},
	bson.D{
		{Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "doujin"},
				{Key: "localField", Value: "_id"},
				{Key: "foreignField", Value: "gallery.id"},
				{Key: "as", Value: "doujin"},
			},
		},
	},
	bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$doujin"}}}},
	bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "$expr",
					Value: bson.D{
						{Key: "$ne",
							Value: bson.A{
								"$doujin.gallery.num_pages",
								"$pages",
							},
						},
					},
				},
			},
		},
	},
}

type GetDoujinWithIncompCache struct {
	ID     int      `bson:"_id"`
	Pages  int      `bson:"pages"`
	Doujin DoujinV2 `bson:"doujin"`
}

var TOTAL_PAGES = bson.A{
	bson.D{{Key: "$project", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$size", Value: "$gallery.images.pages"}}}}}},
	bson.D{
		{Key: "$group",
			Value: bson.D{
				{Key: "_id", Value: primitive.Null{}},
				{Key: "total", Value: bson.D{{Key: "$sum", Value: "$count"}}},
			},
		},
	},
}

var GET_INCOMPLETE_DOUJINS_WITH_PAGES = bson.A{
	bson.D{
		{Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "pages"},
				{Key: "localField", Value: "gallery.id"},
				{Key: "foreignField", Value: "gid"},
				{Key: "as", Value: "pages"},
			},
		},
	},
	bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "$expr",
					Value: bson.D{
						{Key: "$gt",
							Value: bson.A{
								bson.D{{Key: "$size", Value: "$gallery.images.pages"}},
								bson.D{{Key: "$size", Value: "$pages"}},
							},
						},
					},
				},
			},
		},
	},
	bson.D{{Key: "$project", Value: bson.D{{Key: "gallery.tags", Value: 0}}}},
}

type GetIncompleteDoujinsWithPages struct {
	ID        primitive.ObjectID `bson:"_id"`
	Gallery   Gallery            `bson:"gallery"`
	MediaURL  string             `bson:"media_url"`
	StartPage int                `bson:"start_page"`
	Pages     []Page             `bson:"pages"`
}

func (res *GetIncompleteDoujinsWithPages) GetPage(index int) *Page {
	page := res.Gallery.Images.Pages[index]

	if page.skip {
		return nil
	}

	return &Page{
		GalleryID: res.Gallery.ID,
		Page:      index,
		URL:       res.GetPageURL(index),
		DoujinID:  res.ID,
		Name:      fmt.Sprintf("%d+%d.%s", res.Gallery.ID, index, Extensions[page.T]),
	}
}

func (res *GetIncompleteDoujinsWithPages) GetPageURL(index int) string {
	page := res.Gallery.Images.Pages[index]
	return fmt.Sprintf("%sgalleries/%s/%d.%s", res.MediaURL, res.Gallery.MediaID, index+1, Extensions[page.T])
}
