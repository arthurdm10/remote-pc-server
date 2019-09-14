package main

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
