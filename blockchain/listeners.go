package blockchain

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

type ErrorResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

func (bc *Blockchain) OnGetTx(w rest.ResponseWriter, r *rest.Request) {
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
	addr := r.PathParam("address")
	// Get query "type"
	spentFlag := r.FormValue("type")
	// Get qeury "page"
	_ = r.FormValue("page")
	// Get query "sort"
	isSpent := Received
	if spentFlag == "send" {
		isSpent = Send
	}
	txids, err := bc.GetIndexTxs(addr, 0, isSpent)
	if err != nil {
		log.Info(err)
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txids)
}

func res500(msg error, w rest.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	res := ErrorResponse{false, msg.Error()}
	w.WriteJson(res)
}
