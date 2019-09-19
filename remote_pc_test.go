package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestSuitePC(t *testing.T) {

	var (
		key           = "fc58161e6b0da8e0cae8248f40141165"
		adminUser     = fmt.Sprintf("%x", sha256.Sum256([]byte("admin")))
		adminPassword = fmt.Sprintf("%x", sha256.Sum256([]byte("admin")))
		pcUsername    = fmt.Sprintf("%x", sha256.Sum256([]byte("username")))
		pcPassword    = fmt.Sprintf("%x", sha256.Sum256([]byte("passwd")))
	)

	client, err := setupMongodb("localhost:27017")
	assert.Nil(t, err)

	client.Database("test_remote_pc").Drop(context.TODO())

	wsController := NewWsController("admin", "admin", "localhost:27017", "test_remote_pc")
	server := httptest.NewServer(wsController.routes())

	defer server.Close()

	t.Run("RegisterNewPC", func(t *testing.T) {
		url := server.URL + "/create_pc/" + key

		var jsonStr = []byte(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"key": "%s"
		}`, pcUsername, pcPassword, key))

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		authHeader := http.Header{"X-Username": []string{adminUser}, "X-Password": []string{adminPassword}}

		req.Header = authHeader
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Nil(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("FailToRegisterPcAlreadyRegistered", func(t *testing.T) {
		url := server.URL + "/create_pc/" + key

		var jsonStr = []byte(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"key": "%s"
		}`, pcUsername, pcPassword, key))

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		authHeader := http.Header{"X-Username": []string{adminUser}, "X-Password": []string{adminPassword}}

		req.Header = authHeader
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Nil(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("CreateAuthenticatedConnection", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/connect/" + key

		authHeader := http.Header{"X-Username": []string{pcUsername}, "X-Password": []string{pcPassword}}
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

}
