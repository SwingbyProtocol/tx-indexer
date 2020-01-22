package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var (
	apiEndpoint = "ws://localhost:9099/ws"
	watchAddr   = "tb1q8afqk8p2en03m0e9dtpyxhyk9emuzr3u2hey04"
)

type Keeper struct {
	conn *websocket.Conn
}

// Using with environment variables
/*
$ source env.sh
$ go run websocket_sample.go
*/

// Request msg sample
/*
{
	"action": "watchTxs",
	"reqid":"3221212f",
	"params": {
		"address": "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ",
		"txid": "",
		"hex": "",
		"type": "",
		"mempool": true,
		"height_from": 0,
		"height_to": 0,
		"time_from": 0,
		"time_to": 0
	}
}
*/

func main() {
	run := os.Getenv("RUN")
	endpoint := os.Getenv("ENDPOINT")
	if endpoint != "" {
		apiEndpoint = endpoint
	}
	address := os.Getenv("ADDR")
	if address != "" {
		watchAddr = address
	}
	log.Infof("RUN: %s ENDPOINT: %s ADDR: %s", run, apiEndpoint, watchAddr)
	conn, _, err := websocket.DefaultDialer.Dial(apiEndpoint, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	k := Keeper{
		conn: conn,
	}
	k.Start()

	// open
	///k.WriteJSON(api.MsgWsReqest{})

	switch run {
	case "WatchAddrReceived":
		k.WatchAddrReceived()
		break
	case "WatchAddrSend":
		k.WatchAddrSend()
		break
	case "GetIndexTxsReceived":
		k.GetIndexTxsReceived()
		break
	case "GetIndexTxsSend":
		k.GetIndexTxsSend()
		break
	case "GetIndexTxsReceivedWithTimeWindow":
		k.GetIndexTxsReceivedWithTimeWindow()
		break
	case "GetIndexTxsSendWithTimeWindow":
		k.GetIndexTxsSendWithTimeWindow()
		break
	case "BroadcastSignedRawTransaction":
		k.BroadcastSignedRawTransaction()
		break
	default:
		k.WatchAddrReceived()
	}
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
}

// GetIndexTxsReceived gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsReceived() {
	mempool, err := strconv.ParseBool(os.Getenv("MEMPOOL"))
	if err != nil {
		mempool = false
	}
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "",      // "" mean used as "received" ( "received" or "send" ),
			Mempool: mempool, // using mempool
		},
	}
	k.WriteJSON(msg)
}

// GetIndexTxsSend gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
func (k *Keeper) GetIndexTxsSend() {
	mempool, err := strconv.ParseBool(os.Getenv("MEMPOOL"))
	if err != nil {
		mempool = false
	}
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address: watchAddr,
			Type:    "send",  // "" mean used as "received" ( "received" or "send" )
			Mempool: mempool, // using mempool
		},
	}
	k.WriteJSON(msg)
}

// GetIndexTxsReceivedWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
// NOTE: time window only support mined txs
func (k *Keeper) GetIndexTxsReceivedWithTimeWindow() {
	// Round end time
	start, err := strconv.ParseInt(os.Getenv("START"), 10, 64)
	if err != nil {
		start = 0
		log.Info(err)
	}
	end, err := strconv.ParseInt(os.Getenv("END"), 10, 64)
	if err != nil {
		end = 0
		log.Info(err)
	}
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "",    // "" mean used as "received" ( "received" or "send" )
			TimeFrom: start, // 0 means "oldest time"
			TimeTo:   end,   // 0 means "latest time"
		},
	}
	k.WriteJSON(msg)
}

// GetIndexTxsSendWithTimeWindow gets all txs for the target address. The required filter parameters are as follows:
// Address (string) : The target address for index
// mempool (bool default false) : whether include txs that are in the memory pool
// Type (string defualt "") : whether txs is outgoing or incoming
// TimeFrom (int64 unixtime) : start of time window period
// TimeTo (int64 unixtime) : end of time window period
// NOTE: time window only support mined txs
func (k *Keeper) GetIndexTxsSendWithTimeWindow() {
	// Round end time
	start, err := strconv.ParseInt(os.Getenv("START"), 10, 64)
	if err != nil {
		start = 0
		log.Info(err)
	}
	end, err := strconv.ParseInt(os.Getenv("END"), 10, 64)
	if err != nil {
		end = 0
		log.Info(err)
	}
	msg := api.MsgWsReqest{
		Action: "getTxs",
		Params: &api.Params{
			Address:  watchAddr,
			Type:     "send", // "" mean used as "received" ( "received" or "send" )
			TimeFrom: start,  // 0 means "oldest time"
			TimeTo:   end,    // 0 means "latest time"
		},
	}
	k.WriteJSON(msg)
}

func (k *Keeper) BroadcastSignedRawTransaction() {
	hex := os.Getenv("HEX")
	msg := api.MsgWsReqest{
		Action: "broadcast",
		Params: &api.Params{
			Address: "",  // Address should be "" when action is broadcast
			Type:    "",  // Type should be "" when action is broadcast
			Hex:     hex, // e.g. 020000000001011cfac0397a34fb0cdf6a84c8131753db2bf9920dfe3be8c976ba02370bb322d30000000000fdffffff01a2c02d00000000001600143f520b1c2accdf1dbf256ac2435c962e77c10e3c0247304402203ca03725cb7db38564e22fc7381d45e28eccf1cb8c7fe33c33e9c69a422a017c02207fd3b62da1f94c44c4d4dece406a9976d0f33ed071a27ca315b364354856e1cf012102f6717e4284ae8778029a16f636a1e6fee788a59f5c368ef49e98276cdbd3a845ebf61800
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
			log.Infof("action: %s msg: %s ", res.Action, res.Message)
			// show txid
			for _, tx := range res.Txs {
				log.Infof("Tx %s confirm %10d minedtime %10d received %10d", tx.Txid, tx.Height, tx.MinedTime, tx.Receivedtime)
			}
		}
	}()
}

func (k *Keeper) WriteJSON(data interface{}) {
	parsed, err := json.Marshal(data)
	if err != nil {
		return
	}
	err = k.conn.WriteMessage(websocket.TextMessage, parsed)
	if err != nil {
		log.Info(err)
	}
}
