package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/gorilla/websocket"
)

type ErrorCode = int

const (
	PermissionDenied ErrorCode = 0x0A
	InvalidArguments ErrorCode = 0x0B
	InternalError    ErrorCode = 0x0C
	InvalidCommand   ErrorCode = 0x0D
)

// check if its a valid request, and return the request type
func validRequest(jsonRequest Json) (string, bool) {
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

	remotePc    *RemotePC
	wsConn      *websocket.Conn
	collection  *mongo.Collection
	userDoc     Json
	permissions Json
}

func (user *User) getConn() *websocket.Conn {
	return user.wsConn
}

// NewUser returns a user only if it exists
func NewUser(username, password string, pc *RemotePC, db *mongo.Database) *User {
	collection := db.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	user := collection.FindOne(ctx, bson.M{"username": username, "password": password, "pc_key": pc.key})

	doc := make(Json)

	if user.Err() != nil {
		log.Printf("User '%s' with password '%s' not found. Error: %s", username, password, user.Err().Error())
		return nil
	}

	err := user.Decode(&doc)
	if err != nil {
		log.Printf("Failed to decode user document. Error: %s", err.Error())
		return nil
	}
	permissions := doc["permissions"].(Json)

	return &User{username: username, remotePc: pc, collection: collection, userDoc: doc, permissions: permissions["commands"].(Json)}
}

//CreateUser registers a new user for the remote PC
func CreateUser(userData Json, remotePcKey string, db *mongo.Database) RegisterError {
	if !jsonContainsKeys(userData, []string{"username", "password"}) {
		return NewRegisterError(http.StatusBadRequest, "Invalid arguments")
	}

	collection := db.Collection("users")

	remotePcKey = strings.TrimSpace(remotePcKey)

	if len(remotePcKey) == 0 {
		return NewRegisterError(http.StatusBadRequest, "Invalid PC key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//check if a PC with this key exists
	findResult := db.Collection("pcs").FindOne(ctx, bson.M{"key": remotePcKey})
	if findResult.Err() != nil {
		return NewRegisterError(http.StatusNotFound, fmt.Sprintf("Could not find a PC with key '%s'", remotePcKey))
	}

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	findResult = collection.FindOne(ctx, bson.M{"username": userData["username"], "pc_key": remotePcKey})

	if findResult.Err() == nil {
		//username already exists
		return NewRegisterError(http.StatusBadRequest, fmt.Sprintf("Username '%s' already exists", userData["username"]))
	}

	userData["pc_key"] = remotePcKey
	userData["permissions"] = Json{"commands": Json{}}

	_, err := collection.InsertOne(ctx, userData)

	if err != nil {
		return NewRegisterError(http.StatusInternalServerError, err.Error())
	}

	return RegisterError{}
}

func (user *User) readRoutine() {
	defer user.remotePc.disconnectUser()

	for {

		msgType, data, err := user.wsConn.ReadMessage()

		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("User disconnected from PC %s", user.remotePc.key)
				break
			}
			log.Printf("Unknown error on user readRountine - %s", err.Error())
			return
		}

		if msgType == websocket.TextMessage {
			var jsonData Json
			err := json.Unmarshal(data, &jsonData)

			if err != nil {
				log.Printf("Failed to parse json data: %s\n", data)
				continue
			}

			requestType, ok := jsonData["type"].(string)

			if !ok {
				ClientWriteJSON(user.remotePc, jsonData)
				continue
			}

			if requestType == "command" {

				if !jsonContainsKeys(jsonData, []string{"cmd", "args"}) {
					ClientWriteJSON(user, Json{"error": "Invalid request"})
					continue
				}

				cmd, ok := jsonData["cmd"].(string)

				if !ok {
					user.sendCmdResponseError(cmd, "Invalid command", InvalidCommand)
					continue
				}

				requestArgs, ok := jsonData["args"].([]interface{})

				if !ok {
					user.sendCmdResponseError(cmd, "Invalid arguments", InvalidArguments)
					continue
				}

				log.Printf("Received command '%s' with args '%v'\n", cmd, requestArgs)
				sanitizedArgs, errorCode := sanitizeRequestArgs(requestArgs)

				if errorCode != 0 {
					user.sendCmdResponseError(cmd, "Error", errorCode)
				}

				if len(sanitizedArgs) != len(requestArgs) {
					break
				}

				jsonData["args"] = sanitizedArgs
				if !user.havePermission(cmd, jsonData["args"].([]interface{})) {
					log.Printf("User doesnt have permission to use command %s with args %s\n", cmd, jsonData["args"])
					user.sendCmdResponseError(cmd, "Permission Denied", PermissionDenied)
					continue
				}
			}

			ClientWriteJSON(user.remotePc, jsonData)
			continue
		}
		ClientWrite(user.remotePc, msgType, data)
	}
}

func (user *User) havePermission(cmd string, args []interface{}) bool {
	// user have all permissions
	if len(user.permissions) == 0 {
		return true
	}

	if _, found := user.permissions[cmd]; found {
		permission := user.permissions[cmd].(Json)
		restrictions := permission["restrictions"].(bson.A)

		if !permission["allow"].(bool) && len(restrictions) == 0 {
			return false
		}

		// any command that interact with a file (download, delete etc...)

		if len(restrictions) > 0 {
			isFileCommand := cmd != "ls_dir"
			for _, requestArg := range args {
				arg, ok := requestArg.(string)

				if !ok {
					// if its not a string, just ignore it
					continue
				}

				for _, res := range restrictions {
					restriction := res.(Json)
					restrictionPath := restriction["path"].(string)

					if isFileCommand {
						requestedPath := filepath.Clean(filepath.Dir(arg))

						if requestedPath == filepath.Clean(restrictionPath) {
							return restriction["allow"].(bool)
						}

						// check if its a subdirectory of the restricted path
						if strings.Index(requestedPath, restrictionPath) == 0 {
							allowSubDir, ok := restriction["allow_subdir"].(bool)

							if !ok {
								return true
							}

							if !allowSubDir {
								log.Printf("Command not allowed on subdir: %s", requestedPath)
							}

							return allowSubDir
						}
					}

					if strings.Index(arg, restrictionPath) == 0 {
						return restriction["allow"].(bool)
					}
					continue
				}
			}
			return permission["allow"].(bool)
		}

	}

	return true
}

func sanitizeRequestArgs(requestArgs []interface{}) ([]interface{}, ErrorCode) {
	// sanitizedArgs := make([]string, len(requestArgs))
	for i, arg := range requestArgs {
		strArg, ok := arg.(string)
		if !ok {
			// // argument must be a string
			continue
		}

		strArg = strings.ReplaceAll(strArg, `../`, "")
		strArg = strings.ReplaceAll(strArg, `./`, "")

		if len(strArg) > 0 {
			requestArgs[i] = filepath.Clean(strArg)
		}
	}

	return requestArgs, 0
}

func (user *User) sendCmdResponseError(cmd, errorMsg string, errorCode ErrorCode) {
	ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": errorCode, "error_msg": errorMsg})
}
