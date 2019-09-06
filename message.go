package main

type Msg struct {
	Action string                 `json:"action"`
	Data   map[string]interface{} `json:"-"`
}
