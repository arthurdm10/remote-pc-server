package main

import "encoding/json"

type RegisterError struct {
	httpStatusResponse int
	errorMsg           string
}

func NewRegisterError(responseStatus int, msg string) RegisterError {
	return RegisterError{responseStatus, msg}
}

func (err RegisterError) Error() string {
	return err.errorMsg
}

func (err RegisterError) ToJsonString() ([]byte, error) {
	data := map[string]string{"error": err.errorMsg}
	return json.Marshal(data)
}
