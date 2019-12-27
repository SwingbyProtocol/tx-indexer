package blockchain

import (
	"net/http"
	"strconv"

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
	// Get qeury "start"
	// end
	end := int64(0)
	start := int64(0)
	endStr := r.FormValue("time_to")
	if endStr != "" {
		parsed, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			log.Info(err)
		}
		end = parsed
	}
	startStr := r.FormValue("time_from")
	if startStr != "" {
		parsed, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			log.Info(err)
		}
		start = parsed
	}

	isSend := Received
	if spentFlag == "send" {
		isSend = Send
	}
	if start != 0 && end != 0 {
		txs, err := bc.GetIndexTxsWithTW(addr, start, end, isSend)
		if err != nil {
			log.Info(err)
			w.WriteHeader(http.StatusOK)
			w.WriteJson([]*Tx{})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.WriteJson(txs)
		return
	}
	txs, err := bc.GetIndexTxs(addr, 0, isSend)
	if err != nil {
		log.Info(err)
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txs)
}

func res500(msg error, w rest.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	res := ErrorResponse{false, msg.Error()}
	w.WriteJson(res)
}
