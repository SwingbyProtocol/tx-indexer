package btc

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

type Index struct {
	In []*Stamp
}

type Stamp struct {
	TxID string
	Time int64
	Vout [][]string
}

type Out struct {
	TxID string
	Time int64
}

type Node struct {
	BlockChain  *BlockChain
	Index       map[string]*Index
	Txs         map[string]*Tx
	Spent       map[string][]string
	PruneBlocks int64
	LocalBlocks int64
	BestBlocks  int64
	Resolver    *resolver.Resolver
	db          *Database
	Status      string
}

func NewNode(uri string, pruneBlocks int64) *Node {
	node := &Node{
		BlockChain:  NewBlockchain(uri),
		Index:       make(map[string]*Index),
		Txs:         make(map[string]*Tx),
		Spent:       make(map[string][]string),
		PruneBlocks: pruneBlocks,
		db:          NewDB("./db"),
	}
	return node
}

func (node *Node) Start() {
	node.BlockChain.StartSync(3 * time.Second)
	node.BlockChain.StartMemSync(10 * time.Second)
	go node.SubscribeTx()
	go node.SubscribeBlock()

	loop(func() error {
		GetMu().RLock()
		mem := node.BlockChain.Mempool
		log.Infof(" Pool -> %7d Spent -> %7d Index -> %7d Tx -> %d", len(mem.Pool), len(node.Spent), len(node.Index), len(node.Txs))
		GetMu().RUnlock()
		return nil
	}, 11*time.Second)
}

func (node *Node) SubscribeTx() {
	for {
		tx := <-node.BlockChain.Mempool.TxChan
		node.UpdateTx(&tx)
	}
}

func (node *Node) UpdateTx(tx *Tx) {
	for _, vin := range tx.Vin {
		key := vin.Txid + "_" + strconv.Itoa(vin.Vout)
		if checkExist(key, node.Spent[key]) == true {
			continue
		}
		node.Spent[key] = append(node.Spent[key], tx.Txid)
	}
	stamp := &Stamp{tx.Txid, tx.ReceivedTime, nil}
	for _, vout := range tx.Vout {
		vout.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
		vout.Txs = []string{}
		stamp.Vout = append(stamp.Vout, nil)
	}
	for _, vout := range tx.Vout {
		if len(vout.ScriptPubkey.Addresses) == 0 {
			continue
		}
		addr := vout.ScriptPubkey.Addresses[0]
		node.updateIndexIn(addr, stamp)
	}
	node.addTx(tx)
}

func (node *Node) SubscribeBlock() {
	for {
		block := <-node.BlockChain.BlockChan
		for _, tx := range block.Txs {
			loadTx := node.getTx(tx.Txid)
			if loadTx != nil {
				loadTx.AddBlockData(&block)
				GetMu().Lock()
				node.Txs[tx.Txid] = loadTx
				GetMu().Unlock()
			} else {
				tx.AddBlockData(&block)
				tx.ReceivedTime = block.Time
				node.BlockChain.Mempool.TxChan <- *tx
			}
		}
	}
}

func (node *Node) updateIndexIn(addr string, stamp *Stamp) {
	if node.Index[addr] == nil {
		node.Index[addr] = &Index{}
	}
	inTxs := node.getIndexStamps(addr, node.Index[addr].In)
	if inTxs[stamp.TxID] == 1 {
		return
	}
	node.Index[addr].In = append(node.Index[addr].In, stamp)
	sortUp(node.Index[addr].In)
}

func (node *Node) getIndexStamps(addr string, stamps []*Stamp) map[string]int {
	added := make(map[string]int)
	for _, stamp := range stamps {
		added[stamp.TxID] = 1
	}
	return added
}

func (node *Node) addTx(tx *Tx) {
	GetMu().Lock()
	node.Txs[tx.Txid] = tx
	GetMu().Unlock()
}

func (node *Node) getTx(txID string) *Tx {
	GetMu().RLock()
	tx := node.Txs[txID]
	GetMu().RUnlock()
	return tx
}

func sortUp(stamps []*Stamp) {
	sort.SliceStable(stamps, func(i, j int) bool { return stamps[i].Time < stamps[j].Time })
}

func loadSend(index *Index) []string {
	sendIDs := []string{}
	for _, in := range index.In {
		for _, out := range in.Vout {
			if out == nil {
				continue
			}
			for _, send := range out {
				sendIDs = append(sendIDs, send)
			}
		}
	}
	return sendIDs
}

func (node *Node) GetIndex(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	index := node.Index[address]
	w.WriteHeader(http.StatusOK)
	w.WriteJson(index)
}

func (node *Node) GetTx(w rest.ResponseWriter, r *rest.Request) {
	txid := r.PathParam("txid")
	tx := node.getTx(txid)
	if tx == nil {
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
	index := node.Index[address]
	if index == nil {
		res500(w, r)
		return
	}
	for _, in := range index.In {
		for i, out := range in.Vout {
			if len(out) != 0 {
				continue
			}
			addresses := node.Txs[in.TxID].Vout[i].ScriptPubkey.Addresses
			if len(addresses) == 0 {
				continue
			}
			if addresses[0] != address {
				continue
			}
			key := in.TxID + "_" + strconv.Itoa(i)
			sendTxs := node.Spent[key]
			if len(sendTxs) == 0 {
				continue
			}
			in.Vout[i] = sendTxs
		}
	}

	resTxs := []*Tx{}
	if spentFlag == "send" {
		sendIDs := loadSend(index)
		for _, sendID := range sendIDs {
			tx := node.Txs[sendID]
			for i, out := range tx.Vout {
				key := tx.Txid + "_" + strconv.Itoa(i)
				sendTxs := node.Spent[key]
				if len(sendTxs) == 0 {
					continue
				}
				out.Spent = true
				out.Txs = sendTxs
			}
			resTxs = append(resTxs, tx)
		}
		sort.Slice(resTxs, func(i, j int) bool { return resTxs[i].ReceivedTime > resTxs[j].ReceivedTime })
	} else {
		for i := len(index.In) - 1; i >= 0; i-- {
			in := index.In[i]
			tx := node.Txs[in.TxID]
			for i, out := range tx.Vout {
				if len(in.Vout[i]) == 0 {
					continue
				}
				out.Txs = in.Vout[i]
				out.Spent = true
			}
			resTxs = append(resTxs, tx)
		}
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
