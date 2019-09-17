package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestSuitePC(t *testing.T) {

	const (
		key           = "fc58161e6b0da8e0cae8248f40141165"
		adminUser     = "username"
		adminPassword = "passwd"
	)

	client, err := setupMongodb("localhost:27017")
	assert.Nil(t, err)

	client.Database("test_remote_pc").Drop(context.TODO())

	wsController := NewWsController(adminUser, adminPassword, "localhost:27017", "test_remote_pc")
	server := httptest.NewServer(wsController.routes())

	defer server.Close()

	t.Run("RegisterNewPC", func(t *testing.T) {
		url := server.URL + "/create_pc"

		var jsonStr = []byte(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"key": "%s"
		}`, adminUser, adminPassword, key))

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Nil(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("FailToRegisterPcAlreadyRegistered", func(t *testing.T) {
		url := server.URL + "/create_pc"

		var jsonStr = []byte(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"key": "%s"
			}`, adminUser, adminPassword, key))

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Nil(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateAuthenticatedConnection", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/connect/" + key

		authHeader := http.Header{"X-Username": []string{"username"}, "X-Password": []string{"passwd"}}
		wsPcConn, response, err := websocket.DefaultDialer.Dial(url, authHeader)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		assert.Nil(t, err)

		defer wsPcConn.Close()
	})

	t.Run("CreateUnauthenticatedConnectionFail", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/connect/" + key

		// authHeader := http.Header{"X-Username": []string{"username"}, "X-Password": []string{"passwd"}}
		_, response, err := websocket.DefaultDialer.Dial(url, nil)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
		assert.NotNil(t, err)

	})

	// t.Run("createNewConnection", func(t *testing.T) {
	// 	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

	// 	ws, response, err := websocket.DefaultDialer.Dial(url, nil)

	// 	assert.Nil(t, err)
	// 	assert.Nil(t, wsController.remotePcs[key].user)
	// 	assert.NotNil(t, ws)
	// 	assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

	// 	assert.Equal(t, key, wsController.remotePcs[key].key)

	// 	ws.Close()
	// })

	// /*
	// 	Verifica se o PC foi removido do controller
	// */
	// t.Run("disconnectPC", func(t *testing.T) {
	// 	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

	// 	ws, response, err := websocket.DefaultDialer.Dial(url, nil)
	// 	assert.Nil(t, err)
	// 	assert.NotNil(t, ws)
	// 	assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

	// 	ws.Close()

	// 	time.Sleep(time.Second * 1)
	// 	_, found := wsController.remotePcs[key]
	// 	assert.Equal(t, false, found)
	// })

	// /*
	// 	Controller nao deve criar uma conexao, caso a key ja exista
	// */
	// t.Run("keyAlreadyExists", func(t *testing.T) {
	// 	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key

	// 	// Cria a primeira conexao
	// 	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	// 	assert.Nil(t, err)

	// 	//Essa conexao deve falhar, pois ja existe um PC com a mesma key
	// 	ws2, response, err := websocket.DefaultDialer.Dial(url, nil)

	// 	assert.NotNil(t, err)
	// 	assert.Nil(t, ws2)
	// 	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	// 	ws.Close()
	// })

}
