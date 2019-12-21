package api

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
)

type API struct {
	rest *REST
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
	OnAddressIndex func(w rest.ResponseWriter, r *rest.Request)
	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnTx func(w rest.ResponseWriter, r *rest.Request)

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
