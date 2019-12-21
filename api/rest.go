package api

import (
	"log"
	"net"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
)

type REST struct {
	api       *rest.Api
	listeners *Listeners
	listen    *net.TCPAddr
}

func NewREST(conf *Config) *REST {

	r := &REST{
		api:       rest.NewApi(),
		listeners: conf.Listeners,
		listen:    conf.RESTListen,
	}

	r.api.Use(&rest.CorsMiddleware{
		OriginValidator: func(origin string, request *rest.Request) bool {
			return true
		},
	})
	restRouter, err := rest.MakeRouter(
		rest.Get("/keep", r.listeners.OnKeep),
		rest.Get("/txs/btc/:address", r.listeners.OnAddressIndex),
		rest.Get("/txs/btc/tx/:txid", r.listeners.OnTx),
	)
	if err != nil {
		log.Fatal(err)
	}
	r.api.SetApp(restRouter)
	return r
}

func (r *REST) Start() {
	go func() {
		err := http.ListenAndServe(r.listen.String(), r.api.MakeHandler())
		if err != nil {
			log.Fatal(err)
		}
	}()
}

/**

func (node *Node) WsHandler(w http.ResponseWriter, r *http.Request) {
	node.ps.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// Time allowed to read the next pong message from the peer.
	pongWait := 30 * time.Second
	//writeWait := 30 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod := (pongWait * 9) / 10
	// Connection
	conn, err := node.ps.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Info("upgrade:", err)
		return
	}
	defer conn.Close()
	client := pubsub.Client{
		ID:         uuid.Must(uuid.NewV4(), nil).String(),
		Connection: conn,
		Mu:         &node.ps.Mu,
	}
	client.Connection.SetPongHandler(func(msg string) error {
		//client.Connection.SetWriteDeadline(time.Now().Add(writeWait))
		client.Connection.SetReadDeadline(time.Now().Add(pongWait))
		log.Info("Received PON from client id: ", client.ID)
		return nil
	})

	node.ps.AddClient(client)

	log.Info("New Client is connected, total: ", len(node.ps.Clients))

	ticker := time.NewTicker(24 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				node.ps.PublishPing(pingPeriod)
			}
		}
	}()

	for {
		_, message, err := client.Connection.ReadMessage()
		if err != nil {
			log.Info("WS:error:", err)
			node.ps.RemoveClient(client)
			log.Info("Removed Client :" + client.ID)
			break
		}
		msg := pubsub.Message{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			errMsg := "Error: This is not correct message payload"
			log.Info(errMsg)
			node.SendWsMsg(&client, msg.Action, errMsg)
			continue
		}
		node.onAction(&client, msg)
	}

}

func (node *Node) onAction(client *pubsub.Client, msg pubsub.Message) {
	switch msg.Action {
	case WATCHTXS:
		if msg.Address == "" {
			errMsg := "Error: Address is not set"
			node.SendWsMsg(client, WATCHTXS, errMsg)
			break
		}
		node.ps.Subscribe(client, msg.Address)
		log.Infof("new subscriber to Address: -> %s %d %s", msg.Address, len(node.ps.Subscriptions[client.ID]), client.ID)
		successMsg := "Success"
		node.SendWsMsg(client, WATCHTXS, successMsg)
		break

	case UNWATCHTXS:
		if msg.Address == "" {
			errMsg := "Error: Address is not set"
			node.SendWsMsg(client, UNWATCHTXS, errMsg)
			break
		}
		log.Infof("Client want to unsubscribe the Address: -> %s %s", msg.Address, client.ID)
		node.ps.Unsubscribe(client, msg.Address)
		successMsg := "Success"
		node.SendWsMsg(client, UNWATCHTXS, successMsg)
		break

	case GETTXS:

		if msg.Address == "" {
			errMsg := "Error: Address is not set"
			node.SendWsMsg(client, GETTXS, errMsg)
			break
		}

		log.Infof("Client want to get txs of index Address: -> %s %s", msg.Address, client.ID)
		resTxs := []*Tx{}

		if msg.Type == "send" {
			resTxs = node.GetTxsFromIndexWithSpent(msg.Address)
		} else {
			resTxs = node.GetTxsFromIndex(msg.Address)
		}
		if msg.TimestampFrom > 0 {
			txsFrom := []*Tx{}
			for _, tx := range resTxs {
				if tx.Receivedtime >= msg.TimestampFrom {
					txsFrom = append(txsFrom, tx)
				}
			}
			resTxs = txsFrom
		}
		if msg.TimestampTo > 0 {
			txsTo := []*Tx{}
			for _, tx := range resTxs {
				if tx.Receivedtime <= msg.TimestampTo {
					txsTo = append(txsTo, tx)
				}
			}
			resTxs = txsTo
		}
		node.SendWsData(client, GETTXS, msg.Address, resTxs)
		break

	default:
		errMsg := "Error: something wrong"
		log.Info(errMsg)
		node.SendWsMsg(client, msg.Action, errMsg)
		break
	}
}

func (node *Node) GetTxsFromIndexWithSpent(address string) []*Tx {
	resTxs := []*Tx{}
	spents, err := node.index.GetSpents(address, node.storage)
	if err != nil {
		return resTxs
	}
	for i := len(spents) - 1; i >= 0; i-- {
		//spents[i].EnableTxSpent(msg.Address, node.storage)
		resTxs = append(resTxs, spents[i])
	}
	return resTxs
}

func (node *Node) GetTxsFromIndex(address string) []*Tx {
	resTxs := []*Tx{}
	txs, err := node.index.GetIns(address, node.storage)
	if err != nil {
		return resTxs
	}
	for i := len(txs) - 1; i >= 0; i-- {
		//txs[i].EnableTxSpent(msg.Address, node.storage)
		resTxs = append(resTxs, txs[i])
	}
	return resTxs
}

func (node *Node) WsPublishMsg(addr string, tx *Tx) {
	payload := WsPayloadTx{"watchTxs", addr, tx}
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Info(err)
	}
	go node.ps.Publish(addr, bytes, nil)
}

func (node *Node) SendWsMsg(client *pubsub.Client, action string, msg string) error {
	payload := WsPayloadMessage{action, msg}
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Info(err)
		return err
	}
	client.Send(bytes)
	return nil
}

func (node *Node) SendWsData(client *pubsub.Client, action string, address string, data []*Tx) error {
	payload := WsPayloadTxs{action, address, data}
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Info(err)
		return err
	}
	client.Send(bytes)
	return nil
}
\

*/
