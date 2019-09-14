package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
}

func (remotePc *RemotePC) getConn() *websocket.Conn {
	return remotePc.conn
}

func NewRemotePc(key string, wsConn *websocket.Conn, wsController *WsController) *RemotePC {
	return &RemotePC{key: key,
		conn:       wsConn,
		controller: wsController,
	}
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

//AuthenticatePC checks if PC exists in the database
func AuthenticatePC(username, password, key string, db *mongo.Database) bool {
	collection := db.Collection("pcs")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := collection.FindOne(ctx, bson.M{"username": username, "password": password, "key": key})

	return result.Err() == nil
}

// CreateRemotePC creates a user for the remote pc
func CreateRemotePC(authData Json, db *mongo.Database) RegisterError {
	if len(authData) == 3 {
		if !jsonContainsKeys(authData, []string{"username", "password", "key"}) {
			return NewRegisterError(http.StatusBadRequest, "invalid request")
		}

		collection := db.Collection("pcs")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pcKey := authData["key"].(string)

		if AuthenticatePC(authData["username"].(string), authData["password"].(string), pcKey, db) {
			//PC already registered
			log.Printf("PC with key %s already registered!\n", pcKey)
			return NewRegisterError(http.StatusForbidden, fmt.Sprintf("pc with key %s already registered", pcKey))
		}

		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err := collection.InsertOne(ctx, authData)
		if err != nil {
			return NewRegisterError(http.StatusInternalServerError, err.Error())
		}

		return RegisterError{}
	}

	return NewRegisterError(http.StatusBadRequest, "invalid request")
}
