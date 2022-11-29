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

func InsertToDoujinCollection(doujin *Doujin, ctx context.Context) error {
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

func InsertDoujinPage(doujin_id primitive.ObjectID, page *Page, ctx context.Context) error {
	if disable {
		return nil
	}

	coll := GetDBInstance().Collection("pages")
	res, err := coll.InsertOne(ctx, page)

	if err != nil {
		return err
	}

	page.DoujinID = doujin_id
	page.ID = res.InsertedID.(primitive.ObjectID)

	return nil
}

func InsertManyDoujinPages(doujin_id primitive.ObjectID, pages *[]Page, ctx context.Context) (error, []primitive.ObjectID) {
	if disable {
		return nil, []primitive.ObjectID{}
	}

	coll := GetDBInstance().Collection("pages")
	interfaces := []interface{}{}

	for _, p := range *pages {
		interfaces = append(interfaces, p)
	}

	res, err := coll.InsertMany(ctx, interfaces)

	if err != nil {
		return err, []primitive.ObjectID{}
	}

	var ids []primitive.ObjectID

	for _, i := range res.InsertedIDs {
		ids = append(ids, i.(primitive.ObjectID))
	}

	return nil, ids
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

func FindDoujin(ctx context.Context, filter interface{}, opts *options.FindOptions) (*[]Doujin, error) {
	var DoujinArr []Doujin

	cur, err := GetDBInstance().Collection("doujin").Find(ctx, filter, opts)

	if err != nil {
		return nil, err
	}

	return nil, cur.All(ctx, &DoujinArr)
}

func DoujinExists(ctx context.Context, title string, url string) (bool, error) {
	if _, ok := doujin_cache_map[url]; ok {
		return true, nil
	}

	count, err := GetDBInstance().Collection("doujin").CountDocuments(ctx, bson.D{{Key: "title", Value: title}, {Key: "url", Value: url}})
	return count > 0, err
}

func InitCache(ctx context.Context) {
	// Get all in the collection
	opts := options.Find().SetProjection(bson.D{{Key: "pages", Value: 0}, {Key: "tags", Value: 0}})
	all, err := FindDoujin(context.TODO(), bson.M{}, opts)

	check(err)

	for _, d := range *all {
		doujin_cache_map[d.URL] = &d
	}
}
