package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	wsController := WsController{remotePcs: make(map[string]*RemotePC), disconnectPcChan: make(chan string)}

	rand.Seed(time.Now().UnixNano())

	router := mux.NewRouter()

	router.HandleFunc("/create/{key}", wsController.newPcHandler)   // new PC connected
	router.HandleFunc("/access/{key}", wsController.newUserHandler) // user connect to a PC

	http.Handle("/", router)
	go wsController.disconnectPC()

	log.Fatal(http.ListenAndServe(":9002", nil))
}
