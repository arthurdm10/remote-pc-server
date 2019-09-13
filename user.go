package main

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"regexp"
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

			requestType, found := jsonData["type"]

			if !found {
				ClientWriteJSON(user.remotePc, jsonData)
				continue
			}

			if requestType.(string) == "command" {
				cmd := jsonData["cmd"].(string)
				requestArgs := jsonData["args"].([]interface{})
				log.Printf("Received command '%s' with args '%v'\n", cmd, requestArgs)
				sanitizedArgs, errorCode := sanitizeRequestArgs(requestArgs)

				if errorCode != 0 {
					ClientWriteJSON(user, Json{"cmd_response": cmd, "error_code": PermissionDenied, "error_msg": "Error"})
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

				jsonData["args"] = sanitizedArgs
				ClientWriteJSON(user.remotePc, jsonData)
			}
		} else if msgType == websocket.BinaryMessage {
			ClientWrite(user.remotePc, msgType, data)
		}
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

					// /home/frost/flutter/test/file.txt
					// /home/frost/flutter/test

					requestedPath := arg
					if isFileCommand {
						requestedPath = filepath.Dir(arg)
					}

					if requestedPath == restrictionPath {
						return restriction["allow"].(bool)
					}

					if isFileCommand {
						fileExt := filepath.Ext(arg)
						if fileExt == "" {
							//File doesnt have an extension, so let the remote PC decide
							log.Printf("File doesnt have an extension %s, so let the remote PC decide", arg)
							return true
						}

						//its a file command (delete, download, rename)

						//if it does have a extension, check if its in an allowed subdirectory

						//check if file is in  a subdirectory of the restricted path
						if strings.Index(requestedPath, restrictionPath) == 0 {
							allowSubDir, ok := restriction["allow_subdir"].(bool)

							if !ok {
								allowSubDir = true
							}

							log.Printf("Command allowed on subdir: %s == %v", requestedPath, allowSubDir)
							return allowSubDir
						}
					}
				}
			}
			return permission["allow"].(bool)
		}

	}

	return true
}

func sanitizeRequestArgs(requestArgs []interface{}) ([]string, ErrorCode) {
	// sanitize args
	sanitizedArgs := make([]string, len(requestArgs))
	for i, arg := range requestArgs {
		strArg, ok := arg.(string)
		if !ok {
			// // argument must be a string
			log.Printf("Invalid argument '%v' of type '%T'. It must be a string\n", arg, arg)
			return nil, InvalidArguments
		}

		re, err := regexp.Compile(`(^\.\.)|(^\.)|(\/\.\.)|(\/\.)`)

		if err != nil {
			return nil, InternalError
		}

		strArg = string(re.ReplaceAll([]byte(arg.(string)), []byte("")))
		sanitizedArgs[i] = strArg
	}

	return sanitizedArgs, 0
}
