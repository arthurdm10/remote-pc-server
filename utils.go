package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func requestBodyToJson(body io.ReadCloser) (map[string]interface{}, error) {
	decoder := json.NewDecoder(body)
	jsonData := make(map[string]interface{})
	err := decoder.Decode(&jsonData)

	return jsonData, err
}

func jsonContainsKeys(jsonData map[string]interface{}, keys []string) bool {
	for _, key := range keys {
		_, found := jsonData[key]
		if !found {
			return false
		}
	}

	return true
}

func httpBadRequest(response http.ResponseWriter) {
	response.WriteHeader(http.StatusBadRequest)
}

func getAuthHeaders(req *http.Request) (string, string) {
	username := strings.TrimSpace(req.Header.Get(http.CanonicalHeaderKey("x-username")))
	password := strings.TrimSpace(req.Header.Get(http.CanonicalHeaderKey("x-password")))
	return username, password
}
