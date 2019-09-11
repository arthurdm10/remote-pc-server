package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	var mongoDbHost string

	flag.StringVar(&mongoDbHost, "mongodb-host", "localhost:27017", "host:port")

	mongoClient, err := setupMongodb(mongoDbHost)

	if err != nil {
		log.Fatalf("Failed to create connection with mongodb: %s", err.Error())
	}

	wsController := NewWsController(mongoClient)

	rand.Seed(time.Now().UnixNano())

	router := mux.NewRouter()

	router.HandleFunc("/create/{key}", wsController.newPcHandler)   // new PC connected
	router.HandleFunc("/access/{key}", wsController.newUserHandler) // user connect to a PC
	router.HandleFunc("/create_user/{key}", wsController.createUser)
	http.Handle("/", router)
	go wsController.disconnectPC()

	log.Fatal(http.ListenAndServe(":9002", nil))
}

func setupMongodb(mongoDbHost string) (*mongo.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoDbHost))

	if err != nil {
		return nil, err
	}

	// check if its connected
	ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
	err = client.Ping(ctx, readpref.Primary())

	if err != nil {
		return nil, err
	}

	return client, nil
}
