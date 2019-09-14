package main

import (
	"context"
	"encoding/json"
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

func (user *User) getConn() *websocket.Conn {
	return user.wsConn
}

func (user *User) readRoutine() {
	defer user.remotePc.disconnectUser()

	for {

		msgType, data, err := user.wsConn.ReadMessage()

		if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
			break
		}

		// ClientWrite(user.remotePc, msgType, data)

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
				cmd, ok := jsonData["cmd"].(string)

				if !ok {
					ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": InvalidCommand, "error_msg": "Invalid command"})
					continue
				}

				requestArgs, ok := jsonData["args"].([]interface{})

				if !ok {
					ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": InvalidArguments, "error_msg": "Invalid Arguments"})
					continue
				}

				log.Printf("Received command '%s' with args '%v'\n", cmd, requestArgs)
				sanitizedArgs, errorCode := sanitizeRequestArgs(requestArgs)

				if errorCode != 0 {
					ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": errorCode, "error_msg": "Error"})
				}

				if len(sanitizedArgs) != len(requestArgs) {
					break
				}

				jsonData["args"] = sanitizedArgs
				if !user.havePermission(cmd, jsonData["args"].([]string)) {
					log.Printf("user doesnt have permission to use command %s with args %s\n", cmd, jsonData["args"])
					ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": PermissionDenied, "error_msg": "Permission denied"})
					continue
				}

				ClientWriteJSON(user.remotePc, jsonData)
			}

			continue
		}
		ClientWrite(user.remotePc, msgType, data)
	}
}

func (user *User) havePermission(cmd string, args []string) bool {
	if len(user.permissions) == 0 {
		// user have all permissions
		return true
	}
	if _, found := user.permissions[cmd]; found {
		permission := user.permissions[cmd].(Json)
		restrictions := permission["restrictions"].(bson.A)

		if !permission["allow"].(bool) && len(restrictions) == 0 {
			return false
		}

		// log.Printf("User permissions: %s", permission)

		isFileCommand := cmd != "ls_dir"

		if len(restrictions) > 0 {
			for _, arg := range args {
				for _, res := range restrictions {
					restriction := res.(Json)
					restrictionPath := restriction["path"].(string)

					if isFileCommand {
						requestedPath := filepath.Dir(arg)

						if requestedPath == restrictionPath {
							return restriction["allow"].(bool)
						}

						fileExt := filepath.Ext(arg)

						if fileExt == "" {
							// log.Printf("File doesnt have an extension %s, so let the remote PC decide", arg)
							return true
						}

						//its a file command (delete, download, rename)

						//if it does have a extension, check if its in an allowed subdirectory
						if strings.Index(requestedPath, restrictionPath) == 0 {
							allowSubDir, ok := restriction["allow_subdir"].(bool)

							if !ok {
								allowSubDir = true
							}

							log.Printf("Command allowed on subdir: %s == %v", requestedPath, allowSubDir)
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

func sanitizeRequestArgs(requestArgs []interface{}) ([]string, ErrorCode) {
	sanitizedArgs := make([]string, len(requestArgs))
	for i, arg := range requestArgs {
		strArg, ok := arg.(string)
		if !ok {
			// // argument must be a string
			log.Printf("Invalid argument '%v' of type '%T'. It must be a string\n", arg, arg)
			sanitizedArgs[i] = strArg
			continue
		}

		strArg = strings.ReplaceAll(strArg, `../`, "")
		strArg = strings.ReplaceAll(strArg, `./`, "")

		sanitizedArgs[i] = filepath.Clean(strArg)
	}

	return sanitizedArgs, 0
}

func CreateUser(userData Json, remotePcKey string, db *mongo.Database) RegisterError {
	if !jsonContainsKeys(userData, []string{"username", "password"}) {
		return NewRegisterError(http.StatusBadRequest, "Invalid arguments")
	}

	collection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	remotePcKey = strings.TrimSpace(remotePcKey)

	if len(remotePcKey) == 0 {
		return NewRegisterError(http.StatusBadRequest, "Invalid PC key")
	}

	userData["pc_key"] = remotePcKey
	userData["permissions"] = Json{"commands": Json{}}

	_, err := collection.InsertOne(ctx, userData)

	if err != nil {
		return NewRegisterError(http.StatusInternalServerError, err.Error())
	}

	return RegisterError{}
}
