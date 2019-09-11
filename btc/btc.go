package btc

import (
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

var lock = sync.RWMutex{}

type BTCNode struct {
	Index         map[string]*Meta
	Txs           map[string]*Tx
	PruneBlocks   int64
	BestBlockHash string
	BestBlocks    int64
	Chain         string
	LocalBlocks   int64
	Headers       int64
	Resolver      *resolver.Resolver
	URI           string
	isStart       bool
}

type Meta struct {
	Height int64
	Count  int
	Txs    []string
}

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

type Block struct {
	Hash              string `json:"hash"`
	Confirmations     int64  `json:"confirmations"`
	Height            int64  `json:"height"`
	NTx               int64  `json:"nTx"`
	Txs               []*Tx  `json:"tx"`
	Previousblockhash string `json:"previousblockhash"`
}

func NewBTCNode(uri string, pruneBlocks int64) *BTCNode {
	node := &BTCNode{
		Index:       make(map[string]*Meta),
		Txs:         make(map[string]*Tx),
		URI:         uri,
		PruneBlocks: pruneBlocks,
		Resolver:    resolver.NewResolver(),
	}
	return node
}

func (b *BTCNode) Start() {
	go b.doStart()
}

func (b *BTCNode) doStart() {
	ticker := time.NewTicker(10 * time.Second)
	err := b.GetBestBlockHash()
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case <-ticker.C:
			go b.GetBestBlockHash()
		}
	}
}

func (b *BTCNode) GetBestBlockHash() error {
	res := ChainInfo{}
	err := b.Resolver.GetRequest(b.URI, "/rest/chaininfo.json", &res)
	if err != nil {
		log.Info(err)
		return err
	}
	b.BestBlockHash = res.BestBlockHash
	if b.LocalBlocks == 0 {
		b.LocalBlocks = res.Blocks - b.PruneBlocks
	}
	if b.BestBlocks != res.Blocks && b.isStart == false {
		b.BestBlocks = res.Blocks
		b.isStart = true
		err := b.GetBlock(b.BestBlockHash)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BTCNode) GetBlock(bestblockhash string) error {
	res := Block{}
	err := b.Resolver.GetRequest(b.URI, "/rest/block/"+bestblockhash+".json", &res)
	if err != nil {
		b.GetBlock(bestblockhash)
		return err
	}
	if res.Height == 0 {
		b.GetBlock(bestblockhash)
		return errors.New("height is zero")
	}
	for _, tx := range res.Txs {
		for _, vout := range tx.Vout {
			for _, addr := range vout.ScriptPubkey.Addresses {
				b.AddIndexEntry(*addr, res.Height, tx.Txid, vout)
			}
		}
		tx.Confirms = res.Height
		b.Txs[tx.Txid] = tx
	}
	if b.LocalBlocks == res.Height {
		b.LocalBlocks = b.BestBlocks
		b.isStart = false
		err := b.UpdateIndex()
		if err != nil {
			return err
		}
		log.Infof("index count -> %3d syncing -> %5d purne blocks -> %3d", len(b.Index), b.LocalBlocks, b.PruneBlocks)
		return nil
	}
	if b.LocalBlocks < res.Height {
		go b.GetBlock(res.Previousblockhash)
	}
	return nil
}

func (b *BTCNode) UpdateIndex() error {
	if len(b.Index) == 0 {
		return errors.New("index is empty")
	}
	lock.RLock()
	for key, meta := range b.Index {
		if b.LocalBlocks >= meta.Height+b.PruneBlocks {
			delete(b.Index, key)
		}
	}
	lock.RUnlock()
	return nil
}

func (b *BTCNode) AddIndexEntry(addr string, height int64, txID string, vout *Vout) error {
	meta := Meta{}
	count := 0
	lock.RLock()
	if b.Index[addr] != nil {
		meta = *b.Index[addr]
		for _, tx := range meta.Txs {
			if tx == txID {
				lock.RUnlock()
				return errors.New("tx is already added")
			}
		}
	}
	newTxs := append(meta.Txs, txID)
	count = meta.Count + 1
	lock.RUnlock()
	lock.Lock()
	b.Index[addr] = &Meta{
		Height: b.BestBlocks,
		Count:  count,
		Txs:    newTxs,
	}
	lock.Unlock()
	log.Infof("%70s index -> %12d -> txs -> %4d Height -> %d BestHeight -> %d", addr, len(b.Index), len(b.Index[addr].Txs), height, b.BestBlocks)
	return nil
}

func (b *BTCNode) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	sortFlag := r.FormValue("sort")
	txRes := []Tx{}
	lock.RLock()
	if b.Index[address] == nil {
		lock.RUnlock()
		w.WriteHeader(http.StatusInternalServerError)
		res := []string{}
		w.WriteJson(res)
		return
	}
	txs := b.Index[address].Txs
	for _, tx := range txs {
		if b.Txs[tx] == nil {
			continue
		}
		t := *b.Txs[tx]
		txRes = append(txRes, t)
	}
	lock.RUnlock()
	if sortFlag == "asc" {
		sort.SliceStable(txRes, func(i, j int) bool {
			return txRes[i].Confirms < txRes[j].Confirms
		})
	} else {
		sort.SliceStable(txRes, func(i, j int) bool {
			return txRes[i].Confirms > txRes[j].Confirms
		})
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txRes)
}
