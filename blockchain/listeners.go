package blockchain

import (
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func (bc *Blockchain) OnGetTxs(w rest.ResponseWriter, r *rest.Request) {
	// Get path params "txid"
	txid := r.PathParam("txid")
	tx, err := bc.txStore.GetTx(txid)
	if err != nil {
		res500(err, w)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(tx)
}

func (bc *Blockchain) OnGetAddressIndex(w rest.ResponseWriter, r *rest.Request) {
	// Get path params "txid"
	txid := r.PathParam("txid")
	// Get query "type"
	spentFlag := r.FormValue("type")
	// Get qeury "page"
	pageFlag := r.FormValue("page")
	// Get query "sort"

	log.Info(spentFlag, pageFlag)
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txid)
}

func (bc *Blockchain) OnGetTxsWS(c *pubsub.Client) {
	test := []string{"sss"}
	c.SendJSON(test)
}

func res500(msg error, w rest.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	res := ErrorResponse{false, msg.Error()}
	w.WriteJson(res)
}
