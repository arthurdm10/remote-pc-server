package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func ok(err error) bool { return err == nil }

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }}

// WsController its just to keep track of connected PCs
type WsController struct {
	remotePcs        map[string]*RemotePC
	disconnectPcChan chan string //will be used to remove/disconnect remote PCs
	db               *mongo.Database
}

// NewWsController creates a new websocket controller
func NewWsController(db *mongo.Database) *WsController {

	return &WsController{
		remotePcs:        make(map[string]*RemotePC),
		disconnectPcChan: make(chan string),
		db:               db,
	}
}

/**
Handles a new PC connection
If the key doesnt already exists, create a new PC instance and add to controller

TODO: Create a key validation
*/
/// Register a new PC with username, password and a key
func (wsController *WsController) registerRemotePc() http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		pcAuthData, err := requestBodyToJson(req.Body)
		if err != nil {
			log.Printf("Failed to parse pc auth data -- %s\n", err.Error())
			httpBadRequest(response)
			return
		}
		if len(pcAuthData) == 3 {
			if !jsonContainsKeys(pcAuthData, []string{"username", "password", "key"}) {
				httpBadRequest(response)
				return
			}

			collection := wsController.db.Collection("pcs")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			result := collection.FindOne(ctx, bson.M{"key": pcAuthData["key"]})
			if result.Err() == nil {
				//PC already registered
				log.Printf("PC with key %s already registered!\n", pcAuthData["key"])
				response.WriteHeader(http.StatusForbidden)
				return
			}

			ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			collection.InsertOne(ctx, pcAuthData)
			response.WriteHeader(http.StatusCreated)
			return
		}

		httpBadRequest(response)
	}
}

func (wsController *WsController) newRemotePcConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		remotePcKey := mux.Vars(req)["key"]

		// check if the key already exists
		if _, found := wsController.remotePcs[remotePcKey]; !found {
			wsConn, err := upgrader.Upgrade(w, req, nil)
			if ok(err) {
				remotePc := NewRemotePc(remotePcKey, wsConn, wsController)
				wsController.remotePcs[remotePcKey] = remotePc
				log.Printf("new remotePC %s\n", remotePcKey)
				go remotePc.readRoutine()
				return
			}
			log.Printf("Failed to upgrade websocket connection\nError: %s\n", err.error(0))
		}

		w.WriteHeader(http.StatusInternalServerError)
	}
}

/**
Handles a new User connection

TODO: Create a key validation
*/
func (wsController *WsController) newUserConnection() http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		remotePcKey := mux.Vars(req)["key"]

		username := req.Header.Get(http.CanonicalHeaderKey("x-username"))
		password := req.Header.Get(http.CanonicalHeaderKey("x-password"))

		if len(username) == 0 || len(password) == 0 {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		if remotePc, found := wsController.remotePcs[remotePcKey]; found {
			if remotePc.user != nil {
				// PC already have a user connected
				httpBadRequest(response)
				return
			}

			user := NewUser(username, password, remotePc, wsController.db)

			if user == nil {
				response.WriteHeader(http.StatusUnauthorized)
			}
			wsConn, err := upgrader.Upgrade(response, req, nil)
			if ok(err) {
				user.wsConn = wsConn
				remotePc.userConnected(user)
				go user.readRoutine()
				log.Printf("User connected to %s", remotePcKey)
				return
			}

			log.Printf("Failed to upgrade websocket connection\nError: %s\n", err.Error())
		}
		log.Printf("User tryied to connect to %s", remotePcKey)
		response.WriteHeader(http.StatusNotFound)
	}
}

func (wsController *WsController) createUser() http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {

		userData, err := requestBodyToJson(req.Body)

		if err != nil {
			log.Printf("Failed to parse user data\nError:%s\n", err.Error())
			httpBadRequest(response)
			return
		}

		if !jsonContainsKeys(userData, []string{"username", "password"}) {
			httpBadRequest(response)
			return
		}

		collection := wsController.db.Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		remotePcKey := strings.TrimSpace(mux.Vars(req)["key"])

		if len(remotePcKey) == 0 {
			httpBadRequest(response)
			return
		}

		userData["pc_key"] = remotePcKey
		userData["permissions"] = Json{"commands": Json{}}

		_, err = collection.InsertOne(ctx, userData)

		if err != nil {
			log.Printf("Failed to create user -- %s\n", err.Error())
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusCreated)
	}
}

func (wsController *WsController) disconnectPC() {
	defer close(wsController.disconnectPcChan)
	for {
		pcKey := <-wsController.disconnectPcChan

		fmt.Printf("Disconnecting pc: %s\n", pcKey)
		remotePc := wsController.remotePcs[pcKey]
		remotePc.disconnectUser()

		delete(wsController.remotePcs, pcKey)
	}
}

func (wsController *WsController) setUserPermissions() http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		jsonData, err := requestBodyToJson(req.Body)

		if err != nil {
			httpBadRequest(response)
			return
		}

		if !jsonContainsKeys(jsonData, []string{"username", "permissions"}) {
			httpBadRequest(response)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		user := wsController.db.Collection("users").FindOneAndUpdate(ctx,
			bson.M{"username": jsonData["username"]},
			bson.M{"$set": bson.M{"permissions": jsonData["permissions"]}})

		if user.Err() == nil {
			response.WriteHeader(http.StatusOK)
			return
		}
		log.Printf("Failed to set user permissions. Error: %s\n", user.Err().Error())
		response.WriteHeader(http.StatusNotFound)
	}
}

func (wsController *WsController) remotePcOnly(handler http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		remotePcKey := mux.Vars(req)["key"]

		username := strings.TrimSpace(req.Header.Get(http.CanonicalHeaderKey("x-username")))
		password := strings.TrimSpace(req.Header.Get(http.CanonicalHeaderKey("x-password")))

		if len(username) == 0 || len(password) == 0 {
			httpBadRequest(response)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pc := wsController.db.Collection("pcs").FindOne(ctx, bson.M{"key": remotePcKey, "username": username, "password": password})
		if pc.Err() != nil {
			response.WriteHeader(http.StatusForbidden)
			return
		}
		handler(response, req)
	}
}
