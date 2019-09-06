package main

import (
	"sync"

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

	remotePc *RemotePC
	conn     *websocket.Conn

	mutex sync.Mutex
}

func NewUser(username string, wsConn *websocket.Conn, pc *RemotePC) *User {
	return &User{username: username, remotePc: pc, conn: wsConn}
}

func (user *User) getConn() *websocket.Conn {
	return user.conn
}

func (user *User) readRoutine() {
	defer user.remotePc.disconnectUser()

	for {

		msgType, data, err := user.conn.ReadMessage()

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
