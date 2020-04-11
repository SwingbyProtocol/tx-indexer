package api

import (
	"github.com/ant0ine/go-json-rest/rest"
)

type Action struct {
	key         string
	method      string
	HandlerREST func(w rest.ResponseWriter, r *rest.Request)
	HandlerWS   func(c *Client, r *Request)
}

func NewWatch(key string, handler func(c *Client, r *Request)) *Action {
	ac := &Action{
		key:       key,
		method:    "WS:",
		HandlerWS: handler,
	}
	return ac
}

func NewPOST(key string, handler func(w rest.ResponseWriter, r *rest.Request)) *Action {
	ac := &Action{
		key:         key,
		method:      "REST:POST",
		HandlerREST: handler,
	}
	return ac
}

func NewGet(key string, handler func(w rest.ResponseWriter, r *rest.Request)) *Action {
	ac := &Action{
		key:         key,
		method:      "REST:GET",
		HandlerREST: handler,
	}
	return ac
}
