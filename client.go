package main

import (
	"errors"

	"github.com/gorilla/websocket"
)

type Client interface {
	getConn() *websocket.Conn
}

func ClientWriteText(client Client, data []byte) error {
	return client.getConn().WriteMessage(websocket.TextMessage, data)
}
func ClientWriteJSON(client Client, data interface{}) error {
	return client.getConn().WriteJSON(data)
}

func ClientWrite(client Client, msgType int, data []byte) error {
	if client != nil && client.getConn() != nil {
		return client.getConn().WriteMessage(msgType, data)
	}
	return errors.New("Invalid client")
}
