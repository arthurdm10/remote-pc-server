package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/gorilla/websocket"
)

/*
RemotePC - PC that can be accessed remotetly
*/
type RemotePC struct {
	key string

	conn       *websocket.Conn //websocket connection
	user       *User           // current connected user
	controller *WsController
	collection *mongo.Collection //mongodb pcs collection
}

func authenticatePc(username, password, key, string, db *mongo.Database) bool {
	collection := db.Collection("pcs")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := collection.FindOne(ctx, bson.M{"username": username, "password": password, "key": key})

	return result.Err() == nil
}

func NewRemotePc(key string, wsConn *websocket.Conn, wsController *WsController) *RemotePC {
	collection := wsController.db.Collection("pcs")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := collection.FindOne(ctx, bson.M{"key": key})

	if result.Err() != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := collection.InsertOne(ctx, bson.M{"key": key})
		if err != nil {
			log.Printf("Failed to create a PC document -- Error: %s\n", err.Error())
			return nil
		}
		log.Printf("Document created for PC %s\n", key)
	} else {
		log.Printf("Document exists for PC %s\n", key)
	}

	return &RemotePC{key: key,
		conn:       wsConn,
		controller: wsController,
		collection: collection,
	}
}

func (remotePc *RemotePC) getConn() *websocket.Conn {
	return remotePc.conn
}

func (remotePc *RemotePC) userConnected(user *User) error {
	remotePc.user = user
	return ClientWriteJSON(remotePc, map[string]interface{}{"type": "info", "code": 0xfc, "data": user.username})
}

func (remotePc *RemotePC) readRoutine() {
	for {
		msgType, data, err := remotePc.conn.ReadMessage()

		if err != nil {
			break
		}
		// if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived, websocket.CloseServiceRestart) {
		// 	break
		// }

		if remotePc.user == nil {
			continue
		}
		ClientWrite(remotePc.user, msgType, data)
	}

	remotePc.controller.disconnectPcChan <- remotePc.key
}

func (remotePc *RemotePC) disconnectUser() {
	if remotePc.user != nil {
		remotePc.user.wsConn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second*10))
		// remotePc.user.conn.Close()

		remotePc.user = nil
		ClientWriteJSON(remotePc, map[string]interface{}{"type": "info", "code": 0x00, "msg": "User disconnected!"})
	}
}
