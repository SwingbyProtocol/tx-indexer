package api

import "github.com/SwingbyProtocol/tx-indexer/types"

type API struct {
	rest *RESTApi
	ws   *Websocket
}

type APIConfig struct {
	RESTListen  string
	WSListen    string
	Listeners   *Listeners
	PushMsgChan chan *types.PushMsg
}

func NewAPI(conf *APIConfig) *API {
	api := &API{
		rest: NewREST(conf),
		ws:   NewWebsocket(conf),
	}
	return api
}

func (api *API) Start() {
	api.rest.Start()
	api.ws.Start()
}

func (api *API) GetRest() *RESTApi {
	return api.rest
}

func (api *API) GetWs() *Websocket {
	return api.ws
}
