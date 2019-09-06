package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

func ok(err error) bool { return err == nil }

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }}

// WsController its just to keep track of connected PCs
type WsController struct {
	remotePcs        map[string]*RemotePC
	mapMutex         sync.Mutex
	disconnectPcChan chan string
}

/**
Handles a new PC connection
If the key doesnt already exists, create a new PC instance and add to controller

TODO: Create a key validation
*/

func (wsController *WsController) newPcHandler(w http.ResponseWriter, req *http.Request) {
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
	}

	w.WriteHeader(http.StatusBadRequest)
}

/**
Handles a new User connection

TODO: Create a key validation
*/
func (wsController *WsController) newUserHandler(response http.ResponseWriter, req *http.Request) {
	remotePcKey := mux.Vars(req)["key"]

	if remotePc, found := wsController.remotePcs[remotePcKey]; found {
		if remotePc.user != nil {
			// PC already have a user connected
			response.WriteHeader(http.StatusBadRequest)
			return
		}

		wsConn, err := upgrader.Upgrade(response, req, nil)
		if ok(err) {
			user := NewUser("usuario", wsConn, remotePc)
			remotePc.user = user
			go user.readRoutine()
			log.Printf("User connected to %s", remotePcKey)
			return
		}
	}
	log.Printf("User tryied to connect to %s", remotePcKey)
	response.WriteHeader(http.StatusNotFound)
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
