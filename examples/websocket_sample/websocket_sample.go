package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var (
	//apiEndpoint = "wss://indexer.swingby.network/ws"
	apiEndpoint = "wss://testnet-indexer.swingby.network/ws"
	watchAddr   = "tb1qu0vk6z9fkqwuxthfnfmkkup5gmvmsu6l4y6l8k"
)

type Keeper struct {
	conn *websocket.Conn
}

// To exec $ ENDPOINT=<endpoint> ADDR=<address> go run examples/websocket_sample/websocket_sample.go
func main() {
	endpoint := os.Getenv("ENDPOINT")
	if endpoint != "" {
		apiEndpoint = endpoint
	}
	address := os.Getenv("ADDR")
	if address != "" {
		watchAddr = address
	}
	conn, _, err := websocket.DefaultDialer.Dial(apiEndpoint, nil)
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

	//k.GetIndexTxsReceived()
	// get index txs for send
	//k.GetIndexTxsSend()
	// get index txs for received with timestamp window
	//k.GetIndexTxsReceivedWithTimeWindow()
	// get index txs for send with timestamp window
	//k.GetIndexTxsSendWithTimeWindow()
	select {}
}

// WatchAddrReceived subscribes the incoming transaction to the
// target address. The pending transaction on the memory pool is
// disabled by default, and tx obtained by P2Pnetwork is sent to the
// subscribers in real time.
func (k *Keeper) WatchAddrReceived() {
	msg := api.MsgWsReqest{
		Action: "watchTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "",   // "" mean used as "received" ( "received" or "send" )
			Mempool: true, // Need to set
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
		    "action": "watchTxs",
		    "params": {
		        "address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		        "txid": "",
		        "type": "",
		        "mempool": true,
		        "height_from": 0,
		        "height_to": 0,
		        "time_from": 0,
		        "time_to": 0
		    }
		}
	*/
}

// WatchAddrSend subscribes the outgoing transaction from the
// target address. The pending transaction on the memory pool is
// disabled by default, and tx obtained by P2Pnetwork is sent to the
// subscribers in real time.
func (k *Keeper) WatchAddrSend() {
	msg := api.MsgWsReqest{
		Action: "watchTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "send", // "" mean used as "received" ( "received" or "send" )
			Mempool: true,   // Need to set
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
		    "action": "watchTxs",
		    "params": {
		        "address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		        "txid": "",
		        "type": "send",
		        "mempool": true,
		        "height_from": 0,
		        "height_to": 0,
		        "time_from": 0,
		        "time_to": 0
		    }
		}
	*/
}

// GetIndexTxsReceived gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsReceived() {
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "", // "" mean used as "received" ( "received" or "send" ),
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
		    "action": "getTxs",
		    "params": {
		        "address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		        "txid": "",
		        "type": "",
		        "mempool": false,
		        "height_from": 0,
		        "height_to": 0,
		        "time_from": 0,
		        "time_to": 0
		    }
		}
	*/
}

// GetIndexTxsSend gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsSend() {
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "send", // "" mean used as "received" ( "received" or "send" )
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
		    "action": "getTxs",
		    "params": {
		        "address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		        "txid": "",
		        "type": "send",
		        "mempool": false,
		        "height_from": 0,
		        "height_to": 0,
		        "time_from": 0,
		        "time_to": 0
		    }
		}
	*/
}

// GetIndexTxsReceivedWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsReceivedWithTimeWindow() {
	// Round end time
	end := time.Now().Add(-2 * time.Hour)
	from := end.Add(-3 * time.Hour)
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "", // "" mean used as "received" ( "received" or "send" )
			Mempool:  false,
			TimeFrom: from.Unix(),
			TimeTo:   end.Unix(), // 0 means "latest time"
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
		    "action": "getTxs",
		    "params": {
		        "address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		        "txid": "",
		        "type": "",
		        "mempool": false,
		        "height_from": 0,
		        "height_to": 0,
		        "time_from": 1577332562,
		        "time_to": 1577343362
			}
		}
	*/
}

// GetIndexTxsSendWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsSendWithTimeWindow() {
	// Round end time
	end := time.Now().Add(-2 * time.Hour)
	from := end.Add(-3 * time.Hour)
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "send", // "" mean used as "received" ( "received" or "send" )
			Mempool:  false,
			TimeFrom: from.Unix(),
			TimeTo:   end.Unix(), // 0 means "latest time"
		},
	}
	k.WriteJSON(msg)
	/*
		MsgWsReqest:
		{
			"action": "getTxs",
			"params": {
				"address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
				"txid": "",
				"type": "send",
				"mempool": false,
				"height_from": 0,
				"height_to": 0,
				"time_from": 1577332501,
				"time_to": 1577343301
			}
		}
	*/
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
	parsed, err := json.Marshal(data)
	if err != nil {
		return
	}
	//out := new(bytes.Buffer)
	//json.Indent(out, parsed, "", "    ")
	//fmt.Println(out.String())
	err = k.conn.WriteMessage(websocket.TextMessage, parsed)
	if err != nil {
		log.Info(err)
	}
}
