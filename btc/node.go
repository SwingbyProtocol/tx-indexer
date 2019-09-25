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

var (
	blockList = []string{}
)

type Node struct {
	BlockChain  *BlockChain
	Index       map[string]*Index
	Txs         map[string]*Tx
	Missed      map[string]int
	PruneBlocks int64
	LocalBlocks int64
	BestBlocks  int64
	Resolver    *resolver.Resolver
	db          *Database
	Status      string
}

type Index struct {
	In  []*Stamp
	Out []*Stamp
}

type Stamp struct {
	TxID string
	Time int64
}

func NewNode(uri string, pruneBlocks int64) *Node {
	node := &Node{
		BlockChain:  NewBlockchain(uri),
		Index:       make(map[string]*Index),
		Txs:         make(map[string]*Tx),
		Missed:      make(map[string]int),
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
		log.Infof(" Pool -> %7d Index -> %7d Tx -> %d", len(mem.Pool), len(node.Index), len(node.Txs))
		GetMu().RUnlock()
		return nil
	}, 11*time.Second)
}

func (node *Node) SubscribeTx() {
	for {
		tx := <-node.BlockChain.Mempool.TxChan
		for _, vin := range tx.Vin {
			txIn := node.getTx(vin.Txid)
			if txIn == nil {
				node.registUnderTx(vin.Txid)
				continue
			}
			for i, vout := range txIn.Vout {
				if i != vin.Vout {
					continue
				}
				vout.Spent = true
				vout.Txs = append(vout.Txs, tx.Txid)
			}
			node.updateIndex(txIn, true)
			GetMu().Lock()
			node.Txs[txIn.Txid] = txIn
			GetMu().Unlock()
		}
		for _, vout := range tx.Vout {
			vout.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
			vout.Txs = []string{}
		}
		node.updateIndex(&tx, false)
		GetMu().Lock()
		node.Txs[tx.Txid] = &tx
		GetMu().Unlock()
	}
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

func (node *Node) updateIndex(tx *Tx, isOut bool) {
	outs := tx.getOutputsAddresses()
	for _, addr := range outs {
		if node.Index[addr] == nil {
			node.Index[addr] = &Index{}
		}
		if isOut == true {
			outTxs := node.getIndexOuts(addr)
			if outTxs[tx.Txid] == 1 {
				continue
			}
			meta := &Stamp{tx.Txid, tx.ReceivedTime}
			node.Index[addr].Out = append(node.Index[addr].Out, meta)
			sortUp(node.Index[addr].Out)
		} else {
			inTxs := node.getIndexIns(addr)
			if inTxs[tx.Txid] == 1 {
				continue
			}
			meta := &Stamp{tx.Txid, tx.ReceivedTime}
			node.Index[addr].In = append(node.Index[addr].In, meta)
			sortUp(node.Index[addr].In)
		}
	}
}

func (node *Node) getIndexOuts(addr string) map[string]int {
	added := make(map[string]int)
	for _, stamp := range node.Index[addr].Out {
		added[stamp.TxID] = 1
	}
	return added
}

func (node *Node) getIndexIns(addr string) map[string]int {
	added := make(map[string]int)
	for _, stamp := range node.Index[addr].In {
		added[stamp.TxID] = 1
	}
	return added
}

func (node *Node) getTx(txID string) *Tx {
	GetMu().RLock()
	tx := node.Txs[txID]
	GetMu().RUnlock()
	return tx
}

func (node *Node) registUnderTx(txID string) error {
	if node.Missed[txID] != 1 {
		node.Missed[txID] = 1
		log.Info("missed -> ", txID)
	}
	return nil
}

func (node *Node) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	//sortFlag := r.FormValue("sort")
	pageFlag := r.FormValue("page")
	spentFlag := r.FormValue("type")
	resTxs := []*Tx{}
	index := node.Index[address]
	if index == nil {
		res500(w, r)
		return
	}
	if spentFlag == "send" {
		outs := index.Out
		for i := len(outs) - 1; i >= 0; i-- {
			resTxs = append(resTxs, node.Txs[outs[i].TxID])
		}
	} else {
		ins := index.In
		for i := len(ins) - 1; i >= 0; i-- {
			resTxs = append(resTxs, node.Txs[ins[i].TxID])
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

func (node *Node) GetBTCIndex(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	index := node.Index[address]
	w.WriteHeader(http.StatusOK)
	w.WriteJson(index)
}

func (node *Node) GetBTCTx(w rest.ResponseWriter, r *rest.Request) {
	txid := r.PathParam("txid")
	tx := node.getTx(txid)
	if tx == nil {
		res500(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(tx)
}

func sortUp(stamps []*Stamp) {
	sort.SliceStable(stamps, func(i, j int) bool { return stamps[i].Time < stamps[j].Time })
}

func res500(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	res := []string{}
	w.WriteJson(res)
}
