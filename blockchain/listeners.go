package blockchain

import (
	"errors"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
)

type ErrorResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

func (bc *Blockchain) OnGetTx(w rest.ResponseWriter, r *rest.Request) {
	// Get path params "txid"
	txid := r.PathParam("txid")
	tx, _ := bc.GetTx(txid)
	if tx == nil {
		res500(errors.New("tx is not exit"), w)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(tx)
}

func (bc *Blockchain) OnGetAddressIndex(w rest.ResponseWriter, r *rest.Request) {
	// Get path params "txid"
	//addr := r.PathParam("address")
	// Get query "type"
	//spentFlag := r.FormValue("type")
	// Define params
	//end := int64(0)
	//start := int64(0)
	//endStr := r.FormValue("time_to")
	//endParsed, err := strconv.ParseInt(endStr, 10, 64)
	//if err != nil {
	//	log.Info(err)
	//} else {
	//end = endParsed
	//}
	//startStr := r.FormValue("time_from")
	//	startParsed, err := strconv.ParseInt(startStr, 10, 64)
	//	if err != nil {
	//log.Info(err)
	//	} else {
	//start = startParsed
	//}
	//	mempool := r.FormValue("mempool")
	//	mm := false
	//	if mempool == "true" {
	//		mm = true
	//	}
	//isSend := Received
	//if spentFlag == "send" {
	//	isSend = Send
	//}
	//txs := bc.GetIndexTxsWithTW(addr, start, end, isSend, mm)

	//w.WriteHeader(http.StatusOK)
	//w.WriteJson(txs)
	//return
}

func res500(msg error, w rest.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	res := ErrorResponse{false, msg.Error()}
	w.WriteJson(res)
}
