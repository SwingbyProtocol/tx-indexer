package btc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/pubsub"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

const (
	WATCHTXS   = "watchTxs"
	UNWATCHTXS = "unwatchTxs"
	GETTXS     = "getTxs"
)

type Node struct {
	blockchain *BlockChain
	index      *Index
	storage    *Storage
	upgrader   *websocket.Upgrader
	ps         *pubsub.PubSub
}

func NewNode(uri string, purneblocks int) *Node {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	node := &Node{
		blockchain: NewBlockchain(uri, purneblocks),
		index:      NewIndex(),
		storage:    NewStorage(),
		ps:         &pubsub.PubSub{},
		upgrader:   &upgrader,
	}
	return node
}

func (node *Node) Start() {
	node.blockchain.StartSync(3 * time.Second)
	node.blockchain.StartMemSync(10 * time.Second)
	go node.SubscribeTx()
	go node.SubscribeBlock()

	loop(func() error {
		GetMu().RLock()
		mem := node.blockchain.mempool
		latestBlock := node.blockchain.GetLatestBlock()
		log.Infof(" Block# -> %d", latestBlock)
		log.Infof(
			" Pool -> %7d Spent -> %7d Index -> %7d Tx -> %7d",
			len(mem.pool),
			len(node.storage.spent),
			len(node.index.stamps),
			len(node.storage.txs),
		)
		log.Info(node.blockchain.blocktimes)
		if len(node.index.lists) >= 6 {
			for _, m := range node.index.lists[:6] {
				count := node.index.counter[m.Address]
				log.Infof("  c: %7d %7d addr: %60s txid: %s", m.Time, count, m.Address, m.Txid)
			}
		}
		GetMu().RUnlock()
		return nil
	}, 11*time.Second)
}

func (node *Node) SubscribeTx() {
	for {
		tx := <-node.blockchain.mempool.waitchan
		node.storage.AddTx(&tx)
		node.index.AddIn(&tx)
		addresses := tx.GetOutputsAddresses()
		for _, addr := range addresses {
			node.WsPublishMsg(addr, &tx)
		}
	}
}

func (node *Node) SubscribeBlock() {
	for {
		block := <-node.blockchain.waitchan
		node.index.RemoveIndexWithTxBefore(node.blockchain, node.storage)
		newTxs := block.UpdateTxs(node.storage)
		count := 0
		for _, tx := range newTxs {
			tx.AddBlockData(&block)
			tx.Receivedtime = block.Time
			node.blockchain.mempool.waitchan <- *tx
			count++
		}
		log.Info("news -> ", count)
	}
}

func (node *Node) GetIndex(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	stamps := node.index.GetStamps(address)
	w.WriteHeader(http.StatusOK)
	w.WriteJson(stamps)
}

func (node *Node) GetTx(w rest.ResponseWriter, r *rest.Request) {
	txid := r.PathParam("txid")
	tx, err := node.storage.GetTx(txid)
	if err != nil {
		res500(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(tx)
}

func (node *Node) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	//sortFlag := r.FormValue("sort")
	pageFlag := r.FormValue("page")
	spentFlag := r.FormValue("type")
	err := node.index.AddVouts(address, node.storage)
	if err != nil {
		res500(w, r)
		return
	}
	resTxs := []*Tx{}
	if spentFlag == "send" {
		spents, err := node.index.GetSpents(address, node.storage)
		if err != nil {
			res500(w, r)
			return
		}
		for i := len(spents) - 1; i >= 0; i-- {
			spents[i].EnableTxSpent(address, node.storage)
			resTxs = append(resTxs, spents[i])
		}
	} else {
		txs, err := node.index.GetIns(address, node.storage)
		if err != nil {
			res500(w, r)
			return
		}
		for i := len(txs) - 1; i >= 0; i-- {
			txs[i].EnableTxSpent(address, node.storage)
			resTxs = append(resTxs, txs[i])
		}
	}

	if len(resTxs) == 0 {
		res500(w, r)
		return
	}

	pageNum, err := strconv.Atoi(pageFlag)
	if err != nil {
		pageNum = 0
	}
	if len(resTxs) >= 100 {
		p := pageNum * 100
		limit := p + 100
		if len(resTxs) < limit {
			p = 100 * (len(resTxs) / 100)
			limit = len(resTxs)
		}
		resTxs = resTxs[p:limit]
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(resTxs)
}

func res500(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	res := []string{}
	w.WriteJson(res)
}

func (node *Node) WsHandler(w http.ResponseWriter, r *http.Request) {
	node.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	c, err := node.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Info("upgrade:", err)
		return
	}
	defer c.Close()
	log.Info("WS:Client Connected")

	client := pubsub.Client{
		ID:         uuid.Must(uuid.NewV4(), nil).String(),
		Connection: c,
	}

	node.ps.AddClient(client)
	log.Info("New Client is connected, total: ", len(node.ps.Clients))

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Info("WS:error:", err)
			node.ps.RemoveClient(client)
			break
		}
		msg := pubsub.Message{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Info("This is not correct message payload")
			continue
		}
		switch msg.Action {
		case WATCHTXS:
			node.ps.Subscribe(&client, msg.Address)
			log.Infof("new subscriber to Address: -> %s %d %s", msg.Address, len(node.ps.Subscriptions), client.ID)
			break
		case UNWATCHTXS:
			log.Infof("Client want to unsubscribe the Address: -> %s %s", msg.Address, client.ID)
			node.ps.Unsubscribe(&client, msg.Address)
			break
		case GETTXS:
			log.Infof("Client want to get txs of index Address: -> %s %s", msg.Address, client.ID)
			resTxs := []*Tx{}

			spentFlag := ""
			if spentFlag == "send" {
				spents, err := node.index.GetSpents(msg.Address, node.storage)
				if err != nil {
					break
				}
				for i := len(spents) - 1; i >= 0; i-- {
					spents[i].EnableTxSpent(msg.Address, node.storage)
					resTxs = append(resTxs, spents[i])
				}
			} else {
				txs, err := node.index.GetIns(msg.Address, node.storage)
				if err != nil {
					break
				}
				for i := len(txs) - 1; i >= 0; i-- {
					txs[i].EnableTxSpent(msg.Address, node.storage)
					resTxs = append(resTxs, txs[i])
				}
			}
			type Payload struct {
				Action  string
				Address string
				Txs     []*Tx
			}
			payload := Payload{"getTxs", msg.Address, resTxs}
			bytes, err := json.Marshal(payload)
			if err != nil {
				log.Info(err)
			}
			client.Send(bytes)
		default:
			break
		}
	}
}

func (node *Node) WsPublishMsg(addr string, tx *Tx) {
	type Payload struct {
		Action  string
		Address string
		Tx      *Tx
	}
	payload := Payload{"watchTxs", addr, tx}
	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Info(err)
	}
	node.ps.Publish(addr, bytes, nil)
}
