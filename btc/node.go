package btc

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

var (
	lock      = sync.RWMutex{}
	blockList = []string{}
)

type Node struct {
	BlockChain  *BlockChain
	Spent       map[string][]string
	Stored      map[string]bool
	PruneBlocks int64
	LocalBlocks int64
	BestBlocks  int64
	Resolver    *resolver.Resolver
	URI         string
	db          *Database
	Status      string
}

type Index struct {
	Txs []*Meta
}

type Meta struct {
	TxID string
	Time int64
	Vout map[string]string
}

func NewNode(uri string, pruneBlocks int64) *Node {
	node := &Node{
		BlockChain:  NewBlockchain(uri),
		Spent:       make(map[string][]string),
		Stored:      make(map[string]bool),
		URI:         uri,
		PruneBlocks: pruneBlocks,
		db:          NewDB("./db"),
	}
	return node
}

func (node *Node) Start() {
	err := node.db.loadData(node)
	if err != nil {
		log.Info(err)
	}
	node.BlockChain.StartSync(3 * time.Second)
	node.BlockChain.StartMemSync(10 * time.Second)
	node.SubscribeSpent()
	node.SubscribeBlock()

	loop(func() error {
		GetMu().RLock()
		mem := node.BlockChain.Mempool
		log.Infof(" Pool -> %7d Spent -> %7d Stored -> %d", len(mem.Pool), len(node.Spent), len(node.Stored))
		GetMu().RUnlock()
		return nil
	}, 11*time.Second)
}

func (node *Node) SubscribeSpent() {
	go func() {
		for {
			tx := <-node.BlockChain.Mempool.TxChan
			for _, vin := range tx.Vin {
				vinKey := vin.Txid + "_" + strconv.Itoa(vin.Vout)
				GetMu().RLock()
				spents := node.Spent[vinKey]
				GetMu().RUnlock()
				if checkExist(tx.Txid, spents) == true {
					continue
				}
				GetMu().Lock()
				node.Spent[vinKey] = append(node.Spent[vinKey], tx.Txid)
				GetMu().Unlock()
				node.db.storeSpent(vinKey, node.Spent[vinKey])
			}
			totalKeys := make(map[string]string)
			for i, vout := range tx.Vout {
				outKey := tx.Txid + "_" + strconv.Itoa(i)
				vout.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
				vout.Txs = []string{}
				if len(vout.ScriptPubkey.Addresses) == 0 {
					log.Debug("debug : len(vout.ScriptPubkey.Addresses) == 0")
					continue
				}
				if len(vout.ScriptPubkey.Addresses) != 1 {
					log.Info("error : len(vout.ScriptPubkey.Addresses) != 1")
					continue
				}
				totalKeys[outKey] = vout.ScriptPubkey.Addresses[0]
			}
			for _, vout := range tx.Vout {
				for _, addr := range vout.ScriptPubkey.Addresses {
					node.updateIndex(tx.Txid, addr, tx.ReceivedTime, totalKeys)
				}
			}
			GetMu().Lock()
			node.Stored[tx.Txid] = true
			GetMu().Unlock()
			//log.Info("stored -> ", tx.Txid)
			node.db.storeTx(tx.Txid, &tx)
		}
	}()
}

func (node *Node) SubscribeBlock() {
	go func() {
		for {
			block := <-node.BlockChain.BlockChan
			for _, tx := range block.Txs {
				GetMu().RLock()
				if node.Stored[tx.Txid] == true {
					GetMu().RUnlock()
					node.updateTx(tx.Txid, &block)
					log.Info("updated -> ", tx.Txid)
					continue
				}
				GetMu().RUnlock()
				log.Info("new -> ", tx.Txid)
				tx.Confirms = block.Height
				tx.ReceivedTime = block.Time
				tx.MinedTime = block.Time
				tx.Mediantime = block.Mediantime
				node.BlockChain.Mempool.TxChan <- *tx
			}
		}
	}()
}

func (node *Node) updateTx(txID string, block *Block) {
	t := make(chan Tx)
	go node.db.LoadTx(txID, t)
	loadTx := <-t
	loadTx.Confirms = block.Height
	loadTx.MinedTime = block.Time
	loadTx.Mediantime = block.Mediantime
	node.db.storeTx(txID, &loadTx)
}

func (node *Node) updateIndex(txID string, addr string, receiveTime int64, totalKeys map[string]string) {
	index, err := node.db.LoadIndex(addr)
	if index == nil || err != nil {
		index = &Index{}
	}
	txs := []string{}
	GetMu().Lock()
	for _, tx := range index.Txs {
		txs = append(txs, tx.TxID)
	}
	if checkExist(txID, txs) == false {
		meta := Meta{
			TxID: txID,
			Time: receiveTime,
			Vout: totalKeys,
		}
		index.Txs = append(index.Txs, &meta)
		sort.SliceStable(index.Txs, func(i, j int) bool {
			return index.Txs[i].Time > index.Txs[j].Time
		})
	}
	node.db.storeIndex(addr, index)
	GetMu().Unlock()
}

func (node *Node) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	//sortFlag := r.FormValue("sort")
	pageFlag := r.FormValue("page")
	spentFlag := r.FormValue("type")
	GetMu().RLock()
	index, err := node.db.LoadIndex(address)
	if err != nil {
		GetMu().RUnlock()
		res500(w, r)
		return
	}
	GetMu().RUnlock()
	if index == nil {
		res500(w, r)
		return
	}
	resTxs := []*Tx{}
	if spentFlag == "send" {
		spents := node.getSpents(index, address)
		resTxs = node.db.LoadTxs(spents)
	} else {
		txs := []string{}
		for _, meta := range index.Txs {
			txs = append(txs, meta.TxID)
		}
		resTxs = node.db.LoadTxs(txs)
	}

	for _, res := range resTxs {
		for _, vout := range res.Vout {
			if checkExist(address, vout.ScriptPubkey.Addresses) == false {
				continue
			}
			outKey := res.Txid + "_" + strconv.Itoa(vout.N)
			GetMu().RLock()
			txs := node.Spent[outKey]
			GetMu().RUnlock()
			if len(txs) == 0 {
				continue
			}
			vout.Spent = true
			vout.Txs = txs
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

func (node *Node) getSpents(index *Index, address string) []string {
	spents := []string{}
	for _, meta := range index.Txs {
		for key, addr := range meta.Vout {
			if addr != address {
				continue
			}
			GetMu().RLock()
			txs := node.Spent[key]
			GetMu().RUnlock()
			if len(txs) != 0 {
				for _, id := range txs {
					if checkExist(id, spents) == false {
						spents = append(spents, id)
					}
				}
			}
		}
	}
	return spents
}

func (node *Node) GetBTCIndex(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	index, err := node.db.LoadIndex(address)
	if err != nil {
		res500(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(index)
}

func (node *Node) GetBTCTx(w rest.ResponseWriter, r *rest.Request) {
	txid := r.PathParam("txid")
	c := make(chan Tx)
	go node.db.LoadTx(txid, c)
	tx := <-c
	if tx.Hash == "" {
		res500(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(tx)
}
func (node *Node) getSpending(from string, txs []*Tx) []string {
	spents := []string{}
	for _, tx := range txs {
		for _, vout := range tx.Vout {
			isMatchAddr := false
			for _, addr := range vout.ScriptPubkey.Addresses {
				if addr == from {
					isMatchAddr = true
				}
			}
			if isMatchAddr == false {
				continue
			}

		}
	}
	return spents
}

func Loop(f func(), t time.Duration) {
	inv := time.NewTicker(t * time.Millisecond)
	go func() {
		f()
		for {
			select {
			case <-inv.C:
				go f()
			}
		}
	}()
}

func splitCheck(str string, txID string) bool {
	txIDs := strings.Split(str, "_")
	isMatch := false
	for _, tx := range txIDs {
		if tx == txID {
			isMatch = true
		}
	}
	return isMatch
}

func res500(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	res := []string{}
	w.WriteJson(res)
}
