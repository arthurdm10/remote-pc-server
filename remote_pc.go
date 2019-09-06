package main

import (
	"time"

	"github.com/gorilla/websocket"
)

/*
RemotePC - PC that can be accessed remotetly
*/
type RemotePC struct {
	key string

	conn *websocket.Conn

	user *User

	controller *WsController
}

func NewRemotePc(key string, wsConn *websocket.Conn, wsController *WsController) *RemotePC {
	return &RemotePC{key: key,
		conn:       wsConn,
		controller: wsController}
}

func (remotePc *RemotePC) getConn() *websocket.Conn {
	return remotePc.conn
}

func (remotePc *RemotePC) readRoutine() {
	for {
		msgType, data, err := remotePc.conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
			break
		}

		if remotePc.user == nil {
			continue
		}
		ClientWrite(remotePc.user, msgType, data)
	}

	remotePc.controller.disconnectPcChan <- remotePc.key
}

func (remotePc *RemotePC) disconnectUser() {
	if remotePc.user != nil {
		remotePc.user.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second*10))
		// remotePc.user.conn.Close()

		remotePc.user = nil
		ClientWriteJSON(remotePc, map[string]interface{}{"type": "info", "code": 0x00, "msg": "User disconnected!"})
	}
}
