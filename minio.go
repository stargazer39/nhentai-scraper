package main

import (
	"log"

	"github.com/minio/minio-go"
)

var minio_client *minio.Client

func SetMinioInstance(instance *minio.Client) {
	minio_client = instance
}

func GetMinioInstance() *minio.Client {
	if minio_client == nil {
		log.Panicln("Minio instance nil")
	}

	return minio_client
}
