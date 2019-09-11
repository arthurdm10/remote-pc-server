package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/gorilla/websocket"
)

func validCmd(cmd string) bool {
	switch cmd {
	case "ls_dir",
		"upload_file",
		"download_file":
		return true
	}

	return false
}

// check if its a valid request, and return the request type
func validRequest(jsonRequest map[string]interface{}) (string, bool) {
	requestType, found := jsonRequest["type"].(string)
	if found {
		if requestType == "info" || requestType == "command" || requestType == "error" {
			return requestType, true
		}
	}

	return "", false
}

type User struct {
	username string

	remotePc   *RemotePC
	wsConn     *websocket.Conn
	collection *mongo.Collection
}

// NewUser returns a user only if it exists
func NewUser(username, password string, pc *RemotePC, db *mongo.Database) *User {
	collection := db.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	user := collection.FindOne(ctx, bson.M{"username": username, "password": password, "pc_key": pc.key})

	if user.Err() != nil {
		log.Printf("User '%s' with password '%s' not found. Error: %s", username, password, user.Err().Error())
		return nil
	}

	return &User{username: username, remotePc: pc, collection: collection}
}

func (user *User) getConn() *websocket.Conn {
	return user.wsConn
}

func (user *User) readRoutine() {
	defer user.remotePc.disconnectUser()

	for {

		msgType, data, err := user.wsConn.ReadMessage()

		if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
			break
		}

		ClientWrite(user.remotePc, msgType, data)

		// if msgType == websocket.TextMessage {
		// 	var jsonData map[string]interface{}
		// 	err := json.Unmarshal(data, &jsonData)

		// 	if err != nil {
		// 		log.Printf("Failed to parse json data: %s\n", data)
		// 		continue
		// 	}

		// 	requestType, found := jsonData["type"]

		// 	if !found {
		// 		ClientWriteJSON(user.remotePc, jsonData)
		// 		continue
		// 	}

		// 	if requestType.(string) == "command" {
		// 		cmd := jsonData["cmd"].(string)
		// 		if validCmd(cmd) {
		// 			// if its a valid command, send it to remotePC
		// 			ClientWriteJSON(user.remotePc, jsonData)
		// 		}
		// 	}
		// } else if msgType == websocket.BinaryMessage {
		// 	ClientWrite(user.remotePc, msgType, data)
		// }
	}
}
