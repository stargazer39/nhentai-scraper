package main

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongo_client *mongo.Client
var doujin_cache_map = make(map[string]*Doujin)
var disable bool = false

func GetDBInstance() *mongo.Database {
	if mongo_client == nil {
		log.Panic("mongo_client is nil")
	}

	return mongo_client.Database("nhentai")
}

func SetDBInstance(instance *mongo.Client) {
	if mongo_client != nil {
		log.Panic("instance already exists")
	}

	mongo_client = instance
}

func InsertToDoujinCollection(doujin *DoujinV2, ctx context.Context) error {
	if disable {
		return nil
	}

	coll := GetDBInstance().Collection("doujin")
	result, err := coll.InsertOne(ctx, *doujin)

	if err != nil {
		return err
	}

	id := result.InsertedID.(primitive.ObjectID)

	doujin.ID = id

	return nil
}

func UpdateDoujin(doujin_id primitive.ObjectID, ctx context.Context, update interface{}) error {
	if disable {
		return nil
	}

	res, err := GetDBInstance().Collection("doujin").UpdateByID(ctx, doujin_id, update)

	if err != nil {
		return err
	}

	if res.MatchedCount <= 0 {
		return fmt.Errorf("no such doujin in the database with id %s", doujin_id.Hex())
	}

	return nil
}

func FindDoujin(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*[]DoujinV2, error) {
	var DoujinArr []DoujinV2

	cur, err := GetDBInstance().Collection("doujin").Find(ctx, filter, opts...)

	if err != nil {
		return nil, err
	}

	return &DoujinArr, cur.All(ctx, &DoujinArr)
}

func DoujinExists(ctx context.Context, title string, url string) (bool, error) {
	if _, ok := doujin_cache_map[url]; ok {
		return true, nil
	}

	count, err := GetDBInstance().Collection("doujin").CountDocuments(ctx, bson.D{{Key: "title", Value: title}, {Key: "url", Value: url}})
	return count > 0, err
}

/* func InitCache(ctx context.Context) {
	// Get all in the collection
	opts := options.Find().SetProjection(bson.D{{Key: "pages", Value: 0}, {Key: "tags", Value: 0}})
	all, err := FindDoujin(context.TODO(), bson.M{}, opts)

	check(err)

	for _, d := range *all {
		doujin_cache_map[d.URL] = &d
	}
}
*/

func InsertToPageCollection(doujin *Page, ctx context.Context) error {
	if disable {
		return nil
	}

	coll := GetDBInstance().Collection("pages")
	result, err := coll.InsertOne(ctx, *doujin)

	if err != nil {
		return err
	}

	id := result.InsertedID.(primitive.ObjectID)

	doujin.ID = id

	return nil
}

func PageExist(ctx context.Context, name string) (bool, error) {
	count, err := GetDBInstance().Collection("pages").CountDocuments(ctx, bson.D{{Key: "name", Value: name}})
	return count > 0, err
}
