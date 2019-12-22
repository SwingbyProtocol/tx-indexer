package api

import (
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/ant0ine/go-json-rest/rest"
)

type API struct {
	rest     *RESTApi
	ws       *Websocket
	pushChan chan *blockchain.Tx
}

type APIConfig struct {
	RESTListen string
	WSListen   string
	Listeners  *Listeners
	PushChan   chan *blockchain.Tx
}

type Listeners struct {
	// OnKeep is invoked when a peer receives a getaddr bitcoin message.
	OnKeep func(w rest.ResponseWriter, r *rest.Request)
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnGetAddressIndex func(w rest.ResponseWriter, r *rest.Request)
	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnGetTx func(w rest.ResponseWriter, r *rest.Request)
	// OnGetWatchTxWS is invoked when api get address watch messsage.
	OnWatchTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetWatchTxWS is invoked when api get address watch messsage.
	OnUnWatchTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetTxs is invoked when api get txs request messsage.
	OnGetTxsWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetTx is invoked when api get tx request messsage.
	OnGetTxWS func(c *pubsub.Client, req *MsgWsReqest)
}

func NewAPI(conf *APIConfig) *API {
	api := &API{
		rest:     NewREST(conf),
		ws:       NewWebsocket(conf),
		pushChan: conf.PushChan,
	}
	return api
}

func (api *API) Start() {
	api.rest.Start()
	api.ws.Start()
	api.handlePushTxWS()
}

func (api *API) GetRest() *RESTApi {
	return api.rest
}

func (api *API) GetWs() *Websocket {
	return api.ws
}

func (api *API) handlePushTxWS() {
	go func() {
		for {
			tx := <-api.pushChan

			addrs := tx.GetOutsAddrs()
			for _, addr := range addrs {
				api.ws.pubsub.PublishJSON(addr, tx)
			}
		}
	}()
}
