package api

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

type RESTApi struct {
	api       *rest.Api
	listen    string
	listeners *Listeners
}

func NewREST(conf *APIConfig) *RESTApi {
	ra := &RESTApi{
		api:       rest.NewApi(),
		listen:    conf.RESTListen,
		listeners: conf.Listeners,
	}
	ra.api.Use(&rest.CorsMiddleware{
		OriginValidator: func(origin string, request *rest.Request) bool {
			return true
		},
	})
	return ra
}

func (ra *RESTApi) Start() {
	if ra.listeners.OnKeep == nil {
		ra.listeners.OnKeep = ra.OnKeep
	}
	if ra.listeners.OnGetAddressIndex == nil {
		ra.listeners.OnGetAddressIndex = ra.OnKeep
	}
	if ra.listeners.OnGetTx == nil {
		ra.listeners.OnGetTx = ra.OnKeep
	}
	restRouter, err := rest.MakeRouter(
		rest.Get("/keep", ra.listeners.OnKeep),
		rest.Get("/txs/btc/mempool-txs", ra.listeners.OnGetMempool),
		rest.Get("/txs/btc/:address", ra.listeners.OnGetAddressIndex),
		rest.Get("/txs/btc/tx/:txid", ra.listeners.OnGetTx),
		rest.Post("/txs/btc/multi-txs", ra.listeners.OnGetTxMulti),
		rest.Post("/txs/btc/broadcast", ra.listeners.OnBroadcast),
	)
	if err != nil {
		log.Fatal(err)
	}
	ra.api.SetApp(restRouter)

	go func() {
		err := http.ListenAndServe(ra.listen, ra.api.MakeHandler())
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Infof("REST api listen: %s", ra.listen)
}

func (ra *RESTApi) OnKeep(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusOK)
	w.WriteJson("status OK")
	return
}
