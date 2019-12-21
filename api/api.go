package api

import (
	"net"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
)

type API struct {
	rest *REST
	ws   *Websocket
}

type Config struct {
	RESTListen *net.TCPAddr
	WSListen   *net.TCPAddr
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

func NewAPI(conf *Config) *API {

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
