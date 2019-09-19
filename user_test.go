package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

const key = "fc58161e6b0da8e0cae8248f40141165"

func setupMongodb(mongoDbHost string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoDbHost))
	defer cancel()

	if err != nil {
		return nil, err
	}

	// check if its connected
	ctx, cancel = context.WithTimeout(context.Background(), 4*time.Second)
	err = client.Ping(ctx, readpref.Primary())
	defer cancel()

	if err != nil {
		return nil, err
	}

	return client, nil
}

func setup(db *mongo.Database) error {

	//create a PC
	pcData := bson.M{"key": key, "username": "username", "password": "passwd"}
	collection := db.Collection("pcs")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	collection.InsertOne(ctx, pcData)

	userPermissions, err := ioutil.ReadFile("permissions.json")
	if err != nil {
		return err
	}
	userData := make(Json)

	err = json.Unmarshal(userPermissions, &userData)

	if err != nil {
		return err
	}

	userData["password"] = "passwd"
	userData["pc_key"] = key
	_, err = db.Collection("users").InsertOne(context.TODO(), userData)
	if err != nil {
		log.Println(err.Error())
	}

	return nil
}

func teardown(db *mongo.Database) {
	db.Drop(context.TODO())
}

func TestSuiteUser(t *testing.T) {
	mongoClient, _ := setupMongodb("localhost:27017")
	db := mongoClient.Database("test_remote_pc")

	defer teardown(db)

	if err := setup(db); err != nil {
		panic(err.Error())
	}

	wsController := NewWsController("test", "test", "localhost:27017", "test_remote_pc")

	server := httptest.NewServer(wsController.routes())
	defer server.Close()

	createRemotePcURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/connect/" + key
	userConnectURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/access/" + key

	authHeader := http.Header{"X-Username": []string{"username"}, "X-Password": []string{"passwd"}}
	// pcAuthHeader := http.Header{"X-Username": []string{"test"}, "X-Password": []string{"test"}}

	//cria um PC remoto
	wsPcConn, response, err := websocket.DefaultDialer.Dial(createRemotePcURL, authHeader)
	assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	assert.Nil(t, err)

	defer wsPcConn.Close()

	t.Run("mustBeAuthenticated", func(t *testing.T) {
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, nil)
		assert.Error(t, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Nil(t, ws)
	})

	t.Run("connectToPC", func(t *testing.T) {
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, authHeader)

		assert.Nil(t, err)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		assert.NotNil(t, ws)

		assert.NotNil(t, wsController.remotePcs[key].user)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		ws.Close()
	})

	/*
		somente uma conexao por PC
		qualquer tentativa de conexao deve falhar, enquanto tiver uma outra conexao ativa
	*/
	t.Run("OneConnectionPerPc", func(t *testing.T) {

		//primeira conexao
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, authHeader)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		assert.NotNil(t, wsController.remotePcs[key].user)

		//segunda conexao -- deve falhar
		newWsConn, response, err := websocket.DefaultDialer.Dial(userConnectURL, authHeader)
		assert.Error(t, err)
		assert.Nil(t, newWsConn)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
		assert.NotNil(t, wsController.remotePcs[key].user)

		ws.Close()
	})

	/*
		Tenta acessar um PC que nao esta conectado
	*/
	t.Run("pcNotConnected", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/access/123bcd"
		ws, response, err := websocket.DefaultDialer.Dial(url, authHeader)
		assert.Error(t, err)
		assert.Nil(t, ws)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})

	t.Run("UserDisconnected", func(t *testing.T) {
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, authHeader)
		assert.Nil(t, err)
		assert.NotNil(t, wsController.remotePcs[key].user)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

		ws.Close()
		time.Sleep(time.Second * 1)
		assert.Nil(t, wsController.remotePcs[key].user)
	})

	// t.Run("userCantListFilesInDisallowedDir", func(t *testing.T) {
	// 	ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, authHeader)

	// 	assert.Nil(t, err)
	// 	assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	// 	assert.NotNil(t, wsController.remotePcs[key].user)

	// 	commandRequest := Json{"type": "command", "cmd": "ls_dir", "args": []string{"/home/test"}, "stream": false}
	// 	err = ws.WriteJSON(commandRequest)

	// 	assert.Nil(t, err)
	// 	jsonResponse := make(Json)

	// 	err = ws.ReadJSON(&jsonResponse)
	// 	assert.Nil(t, err)

	// 	errorMsg, ok := jsonResponse["error_msg"].(string)
	// 	assert.True(t, ok)
	// 	assert.Equal(t, "Permission Denied", errorMsg)

	// 	errorCode, ok := jsonResponse["error_code"].(float64)
	// 	assert.True(t, ok)
	// 	assert.Equal(t, PermissionDenied, int(errorCode))

	// 	ws.Close()
	// })

}
