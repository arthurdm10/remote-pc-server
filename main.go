package main

import (
	"context"
	"flag"
	"log"
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

	router := mux.NewRouter()

	router.HandleFunc("/create_pc", wsController.registerRemotePc())                                               // create new PC
	router.HandleFunc("/connect/{key}", wsController.remotePcOnly(wsController.newRemotePcConnection()))           // PC connected
	router.HandleFunc("/access/{key}", wsController.newUserConnection())                                           // user connect to a PC
	router.HandleFunc("/create_user/{key}", wsController.remotePcOnly(wsController.createUser()))                  // create a new user
	router.HandleFunc("/set_user_permissions/{key}", wsController.remotePcOnly(wsController.setUserPermissions())) // create a new user

	http.Handle("/", router)

	go wsController.disconnectPC()

	log.Fatal(http.ListenAndServe(":9002", nil))
}

func setupMongodb(mongoDbHost string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoDbHost))
	defer cancel()

	if err != nil {
		return nil, err
	}

	// check if its connected
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	err = client.Ping(ctx, readpref.Primary())
	defer cancel()

	if err != nil {
		return nil, err
	}

	return client, nil
}
