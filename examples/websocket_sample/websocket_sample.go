package main

import (
	"encoding/json"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	apiEndpointMainnet = "ws://localhost:9099/ws"
	watchAddr          = "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ"
)

type Keeper struct {
	conn *websocket.Conn
}

func main() {
	conn, _, err := websocket.DefaultDialer.Dial(apiEndpointMainnet, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	k := Keeper{
		conn: conn,
	}
	k.Start()

	// open
	k.WriteJSON(api.MsgWsReqest{})

	k.WatchAddrReceived()

	k.WatchAddrSend()

	k.GetIndexTxsReceived()
	// get index txs for send
	k.GetIndexTxsSend()
	// get index txs for received with timestamp window
	k.GetIndexTxsReceivedWithTimeWindow()
	// get index txs for send with timestamp window
	k.GetIndexTxsSendWithTimeWindow()
	select {}
}

// WatchAddrReceived subscribes the incoming transaction to the
// target address. The pending transaction on the memory pool is
// enabled by default, and tx obtained by P2Pnetwork is sent to the
// subscriber in real time.
func (k *Keeper) WatchAddrReceived() {
	// Send watch tx
	watchAddrReq := api.MsgWsReqest{
		Action: "watchTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "", // "" mean used as "received" ( "received" or "send" )
		},
	}

	k.WriteJSON(watchAddrReq)
}

// WatchAddrSend subscribes the outgoing transaction from the
// target address. The pending transaction on the memory pool is
// enabled by default, and tx obtained by P2Pnetwork is sent to the
// subscriber in real time.
func (k *Keeper) WatchAddrSend() {
	// Send watch tx
	msg := api.MsgWsReqest{
		Action: "watchTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "send", // "" mean used as "received" ( "received" or "send" )
		},
	}

	k.WriteJSON(msg)
}

// GetIndexTxsReceived gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default true) : whether txs is in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// time_from (uint64 unixtime) : start of time window period
// time_to (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsReceived() {
	// Send watch tx
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "", // "" mean used as "received" ( "received" or "send" ),
		},
	}

	k.WriteJSON(msg)
}

// GetIndexTxsSend gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default true) : whether txs is in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// time_from (uint64 unixtime) : start of time window period
// time_to (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsSend() {
	// Send watch tx
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "send", // "" mean used as "received" ( "received" or "send" )
		},
	}

	k.WriteJSON(msg)
}

// GetIndexTxsReceivedWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default true) : whether txs is in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// time_from (uint64 unixtime) : start of time window period
// time_to (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsReceivedWithTimeWindow() {
	// Round end time
	end := time.Now().Add(-2 * time.Hour)
	from := end.Add(-3 * time.Hour)

	// Send watch tx
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "", // "" mean used as "received" ( "received" or "send" )
			Mempool:  false,
			TimeFrom: from.UnixNano(),
			TimeTo:   end.UnixNano(), // 0 means "latest time"
		},
	}

	k.WriteJSON(msg)
}

// GetIndexTxsSendWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default true) : whether txs is in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// time_from (uint64 unixtime) : start of time window period
// time_to (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsSendWithTimeWindow() {
	// Round end time
	end := time.Now().Add(-2 * time.Hour)
	from := end.Add(-3 * time.Hour)

	// Send watch tx
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "send", // "" mean used as "received" ( "received" or "send" )
			Mempool:  false,
			TimeFrom: from.UnixNano(),
			TimeTo:   end.UnixNano(), // 0 means "latest time"
		},
	}

	k.WriteJSON(msg)
}

func (k *Keeper) Start() {
	go func() {
		for {
			_, message, err := k.conn.ReadMessage()
			if err != nil {
				k.conn.Close()
				log.Fatal(err)
				break
			}
			var res api.MsgWsResponse
			err = json.Unmarshal(message, &res)
			if err != nil {
				log.Info(err)
			}
			log.Infof("action %s %s ", res.Action, res.Message)
			// show txid
			for _, tx := range res.Txs {
				log.Info(tx.Txid)
			}

		}
	}()
}

func (k *Keeper) WriteJSON(data interface{}) {
	json, err := json.Marshal(data)
	if err != nil {
		return
	}
	err = k.conn.WriteMessage(websocket.TextMessage, json)
	if err != nil {
		log.Info(err)
	}
}
