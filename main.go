package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Json = map[string]interface{}

func loadEnvVars() (string, string, string, string) {
	mongoDbHost, found := os.LookupEnv("MONGODB_HOST")

	if !found {
		log.Printf("MONGODB_HOST environment variable not found, using default (localhost:27017)")
		mongoDbHost = "localhost:27017"
	} else {
		re := regexp.MustCompile(`(^[a-zA-z\.]+:\d+)`)
		if !re.Match([]byte(mongoDbHost)) {
			log.Printf("Invalid mongodb host: %s\n", mongoDbHost)
			os.Exit(1)
		}
	}

	port, found := os.LookupEnv("PORT")
	if !found {
		log.Printf("PORT environment variable not found, using default (9002)")
		port = "9002"
	} else {
		re := regexp.MustCompile(`\d+`)
		if !re.Match([]byte(port)) {
			log.Printf("Invalid port: %s\n", port)
			os.Exit(1)
		}
	}

	/*
		Admin username and password will be used to register a remote PC and users that can access this PC
	*/
	adminUser, found := os.LookupEnv("ADMIN_USER")
	if !found {
		log.Printf("Required ADMIN_USER environment variable not found")
		os.Exit(1)
	}

	adminPassword, found := os.LookupEnv("ADMIN_PASSWORD")
	if !found {
		log.Printf("Required ADMIN_PASSWORD environment variable not found")
		os.Exit(1)
	}

	return mongoDbHost, port, adminUser, adminPassword
}

func main() {

	mongoDbHost, port, adminUsername, adminPassword := loadEnvVars()
	mongoClient, err := setupMongodb(mongoDbHost)

	if err != nil {
		log.Fatalf("Failed to create connection with mongodb: %s", err.Error())
		os.Exit(1)
	}

	wsController := NewWsController(adminUsername, adminPassword, mongoClient.Database("remote_pc"))

	http.Handle("/", wsController.routes())

	go wsController.disconnectPC()

	fmt.Println("Listening on port: " + port)
	http.ListenAndServe(":"+port, nil)
}

func setupMongodb(mongoDbHost string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoDbHost))
	defer cancel()

	if err != nil {
		return nil, err
	}

	// check if its connected
	ctx, cancel = context.WithTimeout(context.Background(), 4*time.Second)
	err = client.Ping(ctx, readpref.Primary())
	defer cancel()

	if err != nil {
		return nil, err
	}

	return client, nil
}
