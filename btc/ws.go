package btc

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/pubsub"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

const (
	WATCHTXS   = "watchTxs"
	UNWATCHTXS = "unwatchTxs"
	GETTXS     = "getTxs"
)

type WsPayloadTx struct {
	Action  string `json:"action"`
	Address string `json:"address"`
	Tx      *Tx    `json:"tx"`
}

type WsPayloadTxs struct {
	Action  string `json:"action"`
	Address string `json:"address"`
	Tx      []*Tx  `json:"txs"`
}

type WsPayloadMessage struct {
	Action  string `json:"action"`
	Message string `json:"message"`
}

func (node *Node) WsHandler(w http.ResponseWriter, r *http.Request) {
	node.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// Time allowed to read the next pong message from the peer.
	pongWait := 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod := (pongWait * 9) / 10

	conn, err := node.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Info("upgrade:", err)
		return
	}
	defer conn.Close()
	log.Info("WS:Client Connected")
	client := pubsub.Client{
		ID:         uuid.Must(uuid.NewV4(), nil).String(),
		Connection: conn,
	}
	client.Connection.SetPongHandler(func(msg string) error {
		client.Connection.SetReadDeadline(time.Now().Add(pongWait))
		log.Info("Received PON from client id: ", client.ID)
		return nil
	})
	node.ps.AddClient(client)
	log.Info("New Client is connected, total: ", len(node.ps.Clients))

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				node.ps.PublishPing()
			}
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Info("WS:error:", err)
			node.ps.RemoveClient(client)
			break
		}
		msg := pubsub.Message{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			errMsg := "Error: This is not correct message payload"
			log.Info(errMsg)
			payload := WsPayloadMessage{"getTxs", errMsg}
			bytes, err := json.Marshal(payload)
			if err != nil {
				log.Info(err)
			}
			client.Send(bytes)
			break
		}
		switch msg.Action {
		case WATCHTXS:
			node.ps.Subscribe(&client, msg.Address)
			log.Infof("new subscriber to Address: -> %s %d %s", msg.Address, len(node.ps.Subscriptions), client.ID)
			Msg := "Success"
			payload := WsPayloadMessage{WATCHTXS, Msg}
			bytes, err := json.Marshal(payload)
			if err != nil {
				log.Info(err)
			}
			client.Send(bytes)
			break

		case UNWATCHTXS:
			log.Infof("Client want to unsubscribe the Address: -> %s %s", msg.Address, client.ID)
			node.ps.Unsubscribe(&client, msg.Address)
			Msg := "Success"
			payload := WsPayloadMessage{UNWATCHTXS, Msg}
			bytes, err := json.Marshal(payload)
			if err != nil {
				log.Info(err)
			}
			client.Send(bytes)
			break

		case GETTXS:
			log.Infof("Client want to get txs of index Address: -> %s %s", msg.Address, client.ID)
			resTxs := []*Tx{}

			if msg.Type == "send" {
				spents, err := node.index.GetSpents(msg.Address, node.storage)
				if err != nil {
					break
				}
				for i := len(spents) - 1; i >= 0; i-- {
					//spents[i].EnableTxSpent(msg.Address, node.storage)
					resTxs = append(resTxs, spents[i])
				}
			} else {
				txs, err := node.index.GetIns(msg.Address, node.storage)
				if err != nil {
					break
				}
				for i := len(txs) - 1; i >= 0; i-- {
					//txs[i].EnableTxSpent(msg.Address, node.storage)
					resTxs = append(resTxs, txs[i])
				}
			}
			txs := []*Tx{}
			if msg.TimestampFrom > 0 {
				for _, tx := range resTxs {
					if tx.Receivedtime >= msg.TimestampFrom {
						txs = append(txs, tx)
					}
				}
			}
			resTxs = txs

			payload := WsPayloadTxs{"getTxs", msg.Address, resTxs}
			bytes, err := json.Marshal(payload)
			if err != nil {
				log.Info(err)
			}
			client.Send(bytes)
			break

		default:
			break
		}
	}
}

func (node *Node) WsPublishMsg(addr string, tx *Tx) {
	payload := WsPayloadTx{"watchTxs", addr, tx}
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Info(err)
	}
	go node.ps.Publish(addr, bytes, nil)
}
