package main

import (
	"encoding/json"
	"io"
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
