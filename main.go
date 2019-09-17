package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
)

type Json = map[string]interface{}

func loadEnvVars() (string, string, string, string) {
	mongoDbHost, found := os.LookupEnv("MONGODB_HOST")

	if !found {
		log.Printf("MONGODB_HOST not defined, using default (mongo:27017)")
		mongoDbHost = "mongo:27017"
	} else {
		re := regexp.MustCompile(`(^[a-zA-z0-9\.]+:\d+)`)
		if !re.Match([]byte(mongoDbHost)) {
			log.Printf("Invalid mongodb host: %s\n", mongoDbHost)
			os.Exit(1)
		}
	}

	port, found := os.LookupEnv("PORT")
	if !found {
		log.Printf("PORT not defined, using default (9002)")
		port = "9002"
	} else {
		re := regexp.MustCompile(`\d+`)
		if !re.Match([]byte(port)) {
			log.Printf("Invalid port: %s\n", port)
			os.Exit(1)
		}
	}

	/*Admin username and password will be used
	to register a remote PC and users that will access this PC*/
	adminUser, found := os.LookupEnv("ADMIN_USER")
	if !found {
		log.Printf("Required ADMIN_USER environment variable not defined")
		os.Exit(1)
	}

	adminPassword, found := os.LookupEnv("ADMIN_PASSWORD")
	if !found {
		log.Printf("Required ADMIN_PASSWORD environment variable not defined")
		os.Exit(1)
	}

	return mongoDbHost, port, adminUser, adminPassword
}

func main() {

	mongoDbHost, port, adminUsername, adminPassword := loadEnvVars()

	wsController := NewWsController(adminUsername, adminPassword, mongoDbHost, "remote_pc")

	http.Handle("/", wsController.routes())

	go wsController.disconnectPCRoutine()

	fmt.Println("Listening on port: " + port)
	http.ListenAndServe(":"+port, nil)
}
