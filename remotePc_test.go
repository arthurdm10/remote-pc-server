package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestSuitePC(t *testing.T) {
	wsController := WsController{remotePcs: make(map[string]*RemotePC)}
	const key = "abc123"

	router := mux.NewRouter()
	router.HandleFunc("/create/{key}", wsController.newPcHandler)

	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("createNewConnection", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

		ws, response, err := websocket.DefaultDialer.Dial(url, nil)

		assert.Nil(t, err)
		assert.Nil(t, wsController.remotePcs[key].user)
		assert.NotNil(t, ws)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

		assert.Equal(t, key, wsController.remotePcs[key].key)

		ws.Close()
	})

	/*
		Verifica se o PC foi removido do controller
	*/
	t.Run("disconnectPC", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

		ws, response, err := websocket.DefaultDialer.Dial(url, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ws)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

		ws.Close()

		time.Sleep(time.Second * 1)
		_, found := wsController.remotePcs[key]
		assert.Equal(t, false, found)
	})

	/*
		Controller nao deve criar uma conexao, caso a key ja exista
	*/
	t.Run("keyAlreadyExists", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

		// Cria a primeira conexao
		ws, _, err := websocket.DefaultDialer.Dial(url, nil)
		assert.Nil(t, err)

		//Essa conexao deve falhar, pois ja existe um PC com a mesma key
		ws2, response, err := websocket.DefaultDialer.Dial(url, nil)

		assert.NotNil(t, err)
		assert.Nil(t, ws2)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)

		ws.Close()
	})

}
