package api

import (
	"github.com/ant0ine/go-json-rest/rest"
	"net/http"
)

type API struct {
	rest *RESTApi
	ws   *Websocket
}

type APIConfig struct {
	RESTListen string
	WSListen   string
	Listeners  *Listeners
}

type Listeners struct {
	// OnKeep is invoked when a peer receives a getaddr bitcoin message.
	OnKeep func(w rest.ResponseWriter, r *rest.Request)
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnGetAddressIndex func(w rest.ResponseWriter, r *rest.Request)
	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnGetTxs func(w rest.ResponseWriter, r *rest.Request)

	OnWebsocketMsg func(w http.ResponseWriter, r *http.Request)
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
