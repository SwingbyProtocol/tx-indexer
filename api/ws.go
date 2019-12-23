package api

import (
	"encoding/json"
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const (
	WATCHTXS   = "watchTxs"
	UNWATCHTXS = "unwatchTxs"
	GETTXS     = "getTxs"
	GETTX      = "getTx"
)

type Websocket struct {
	pubsub    *pubsub.PubSub
	listen    string
	listeners *Listeners
}

type MsgWsReqest struct {
	Action string  `json:"action"`
	Params *Params `json:"params"`
}

type Params struct {
	Address       string `json:"address"`
	Txid          string `json:"txid"`
	Type          string `json:"type"`
	TimestampFrom int64  `json:"timestamp_from"`
	TimestampTo   int64  `json:"timestamp_to"`
}

type MsgWsResponse struct {
	Action  string      `json:"action"`
	Result  bool        `json:"result"`
	Message string      `json:"message"`
	Txs     interface{} `json:"txs"`
}

func NewWebsocket(conf *APIConfig) *Websocket {
	ws := &Websocket{
		pubsub:    pubsub.NewPubSub(),
		listen:    conf.WSListen,
		listeners: conf.Listeners,
	}
	return ws
}

func (ws *Websocket) Start() {
	go func() {
		http.HandleFunc("/ws", ws.onhandler)
		err := http.ListenAndServe(ws.listen, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Infof("WS api listen: %s", ws.listen)
}

func (ws *Websocket) onhandler(w http.ResponseWriter, r *http.Request) {
	ws.pubsub.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	// Time allowed to read the next pong message from the peer.
	pongWait := 30 * time.Second
	//writeWait := 30 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod := (pongWait * 9) / 10
	// Connection
	conn, err := ws.pubsub.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Info("upgrade:", err)
		conn.Close()
		return
	}

	// Create a pubsub client
	client := pubsub.Client{
		ID:         uuid.Must(uuid.NewRandom()).String(),
		Connection: conn,
		Mu:         &ws.pubsub.Mu,
	}
	// Add PONG handler
	client.Connection.SetPongHandler(func(msg string) error {
		//client.Connection.SetWriteDeadline(time.Now().Add(writeWait))
		client.Connection.SetReadDeadline(time.Now().Add(pongWait))
		log.Info("Received PON from client id: ", client.ID)
		return nil
	})
	// Add message handler
	client.SetMsgHandlers(func(c *pubsub.Client, msg []byte) error {
		// Call onAction
		ws.onAction(c, msg)
		return nil
	})
	// Register pubsub client to pubsub manager
	ws.pubsub.AddClient(client)

	log.Info("New Client is connected, total: ", len(ws.pubsub.Clients))

	ticker := time.NewTicker(24 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				ws.pubsub.PublishPing(pingPeriod)
			}
		}
	}()

}

func (ws *Websocket) GetPubsub() *pubsub.PubSub {
	return ws.pubsub
}

func (ws *Websocket) onAction(c *pubsub.Client, msg []byte) {
	req := MsgWsReqest{}
	err := json.Unmarshal(msg, &req)
	if err != nil {
		errMsg := MsgWsResponse{
			Action:  "",
			Result:  false,
			Message: err.Error(),
			Txs:     []*blockchain.Tx{},
		}
		c.SendJSON(errMsg)
	}

	switch req.Action {
	case WATCHTXS:
		ws.listeners.OnWatchTxWS(c, &req)

	case UNWATCHTXS:
		ws.listeners.OnUnWatchTxWS(c, &req)

	case GETTX:
		ws.listeners.OnGetTxWS(c, &req)

	case GETTXS:
		ws.listeners.OnGetTxsWS(c, &req)
	}
}

func CreateMsgSuccessWS(action string, message string, data interface{}) MsgWsResponse {
	msg := MsgWsResponse{
		Action:  action,
		Result:  true,
		Message: message,
		Txs:     data,
	}
	return msg
}

func CreateMsgErrorWS(action string, errMsg string) MsgWsResponse {
	msg := MsgWsResponse{
		Action:  action,
		Result:  false,
		Message: errMsg,
		Txs:     []string{},
	}
	return msg
}
