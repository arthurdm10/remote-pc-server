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

func TestSuiteUser(t *testing.T) {
	wsController := WsController{remotePcs: make(map[string]*RemotePC)}
	const key = "abc123"

	router := mux.NewRouter()
	router.HandleFunc("/create_pc", wsController.registerRemotePc())                                               // create new PC
	router.HandleFunc("/connect/{key}", wsController.remotePcOnly(wsController.newRemotePcConnection()))           // PC connected
	router.HandleFunc("/access/{key}", wsController.newUserConnection())                                           // user connect to a PC
	router.HandleFunc("/create_user/{key}", wsController.remotePcOnly(wsController.createUser()))                  // create a new user
	router.HandleFunc("/set_user_permissions/{key}", wsController.remotePcOnly(wsController.setUserPermissions())) // create a new user

	server := httptest.NewServer(router)
	defer server.Close()

	createRemotePcURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/create/" + key
	userConnectURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/access/" + key

	//cria um PC remoto
	wsPcConn, _, err := websocket.DefaultDialer.Dial(createRemotePcURL, nil)
	assert.Nil(t, err)

	defer wsPcConn.Close()

	t.Run("createNewConnection", func(t *testing.T) {
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, nil)
		assert.Nil(t, err)
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
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, nil)
		assert.Nil(t, err)
		assert.NotNil(t, wsController.remotePcs[key].user)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

		//segunda conexao -- deve falhar
		newWsConn, response, err := websocket.DefaultDialer.Dial(userConnectURL, nil)
		assert.Error(t, err)
		assert.Nil(t, newWsConn)
		assert.NotNil(t, wsController.remotePcs[key].user)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)

		ws.Close()
	})

	/*
		Tenta acessar um PC que nao esta conectado
	*/
	t.Run("pcNotConnected", func(t *testing.T) {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/access/123bcd"
		ws, response, err := websocket.DefaultDialer.Dial(url, nil)
		assert.Error(t, err)
		assert.Nil(t, ws)
		assert.Equal(t, http.StatusNotFound, response.StatusCode)
	})

	t.Run("UserDisconnected", func(t *testing.T) {
		ws, response, err := websocket.DefaultDialer.Dial(userConnectURL, nil)
		assert.Nil(t, err)
		assert.NotNil(t, wsController.remotePcs[key].user)
		assert.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)

		ws.Close()
		time.Sleep(time.Second * 1)
		assert.Nil(t, wsController.remotePcs[key].user)
	})

	t.Run("AllowOnlyKnownCommands", func(t *testing.T) {
		ws, _, err := websocket.DefaultDialer.Dial(userConnectURL, nil)

		ws.WriteMessage(websocket.TextMessage, []byte("unknown"))
		msgType, data, err := ws.ReadMessage()

		assert.Nil(t, err)
		assert.Equal(t, msgType, websocket.TextMessage)
		assert.Equal(t, "unknown command", string(data))
		ws.Close()

		time.Sleep(time.Second * 1)
		assert.Nil(t, wsController.remotePcs[key].user)
	})

	t.Run("ReceiveResponseFromRemotePC", func(t *testing.T) {
		ws, _, err := websocket.DefaultDialer.Dial(userConnectURL, nil)

		ws.WriteMessage(websocket.TextMessage, []byte("cmd1"))

		msgType, data, err := wsPcConn.ReadMessage()
		wsPcConn.WriteMessage(websocket.TextMessage, []byte("cmd1_response"))

		msgType, data, err = ws.ReadMessage()

		assert.Nil(t, err)
		assert.Equal(t, msgType, websocket.TextMessage)
		assert.Equal(t, "cmd1_response", string(data))
		ws.Close()
	})

}
