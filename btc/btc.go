package btc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	lock = sync.RWMutex{}
)

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
	db            *leveldb.DB
	isStart       bool
	Status        string
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
	db, err := leveldb.OpenFile("./db", nil)
	if err != nil {
		log.Fatal(err)
	}
	node := &BTCNode{
		Index:       make(map[string]*Meta),
		Txs:         make(map[string]*Tx),
		URI:         uri,
		PruneBlocks: pruneBlocks,
		Resolver:    resolver.NewResolver(),
		db:          db,
		Status:      "init",
	}
	return node
}

func (b *BTCNode) Start() {
	err := b.LoadData()
	if err != nil {
		log.Fatal(err)
	}
	b.Run()
}

func (b *BTCNode) LoadData() error {
	iter := b.db.NewIterator(nil, nil)
	log.Info("loading data....")
	underBlocks := int64(0)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		if string(key[:5]) == "index" {
			var meta Meta
			json.Unmarshal(value, &meta)
			b.Index[string(key[6:])] = &meta
		}
		if string(key[:3]) == "txs" {
			var tx Tx
			json.Unmarshal(value, &tx)
			b.Txs[string(key[4:])] = &tx
			if underBlocks == 0 {
				underBlocks = int64(1000000000000)
			}
			if underBlocks > tx.Confirms {
				underBlocks = tx.Confirms
			}
		}
		//log.Info(string(key))
	}
	b.LocalBlocks = underBlocks
	iter.Release()
	err := iter.Error()
	if err != nil {
		log.Info(err)
		return err
	}
	log.Infof("loaded completed. old -> %d index -> %d txs -> %d", b.LocalBlocks, len(b.Index), len(b.Txs))
	return nil
}

func (b *BTCNode) Run() error {
	res := ChainInfo{}
	err := b.Resolver.GetRequest(b.URI, "/rest/chaininfo.json", &res)
	if err != nil {
		time.Sleep(10 * time.Second)
		go b.Run()
		return err
	}
	log.Info("call to bitcoind best blocks -> ", res.Blocks)

	if b.Status == "init" {
		b.Status = "loaded"
		if b.LocalBlocks+b.PruneBlocks <= res.Blocks+1 && b.LocalBlocks != 0 {
			b.BestBlocks = res.Blocks
			b.LocalBlocks = res.Blocks
		} else {
			b.BestBlocks = 0
			b.LocalBlocks = res.Blocks - b.PruneBlocks
		}
	}
	if b.BestBlocks != res.Blocks && b.isStart == false {
		b.BestBlockHash = res.BestBlockHash
		b.BestBlocks = res.Blocks
		b.isStart = true
		err := b.GetBlock(b.BestBlockHash)
		if err != nil {
			log.Info(err)
		}
	}
	time.Sleep(10 * time.Second)
	go b.Run()
	return nil
}

func (b *BTCNode) GetBlock(blockHash string) error {
	res := Block{}
	err := b.Resolver.GetRequest(b.URI, "/rest/block/"+blockHash+".json", &res)
	if err != nil {
		go b.GetBlock(blockHash)
		return err
	}
	if res.Height == 0 {
		go b.GetBlock(blockHash)
		return errors.New("height is zero")
	}
	log.Infof("Fetch block -> %d", res.Height)
	for _, tx := range res.Txs {
		lock.RLock()
		old := b.Txs[tx.Txid]
		lock.RUnlock()
		if old != nil {
			continue
		}
		tx.Confirms = res.Height
		go b.PutTxs(tx.Txid, tx)
		for _, vout := range tx.Vout {
			for _, addr := range vout.ScriptPubkey.Addresses {
				go b.PutIndex(addr, res.Height, tx.Txid)
			}
		}
	}
	if b.LocalBlocks+1 >= res.Height {
		b.LocalBlocks = b.BestBlocks
		b.isStart = false
		b.FinalizeIndex()
		return nil
	}
	if b.LocalBlocks < res.Height {
		go func() {
			time.Sleep(1 * time.Second)
			b.GetBlock(res.Previousblockhash)
		}()
	}
	return nil
}

func (b *BTCNode) FinalizeIndex() {
	if len(b.Index) == 0 {
		return
	}
	removeCountIndex := 0
	removeCountTxs := 0
	lock.RLock()
	for i, meta := range b.Index {
		if meta.Height < b.LocalBlocks-b.PruneBlocks || meta.Height > b.LocalBlocks {
			removeCountIndex = removeCountIndex + 1
			go func() {
				err := b.DeleteIndex(i)
				if err != nil {
					log.Info(err)
				}
			}()
			for _, tx := range meta.Txs {
				removeCountTxs = removeCountTxs + 1
				txID := tx
				go func() {
					err := b.DeleteTxs(txID)
					if err != nil {
						log.Info(err)
					}
				}()
			}
		}
	}
	lock.RUnlock()
	log.Infof("Removed index count -> %3d txs -> %3d", removeCountIndex, removeCountTxs)
	log.Infof("Updated index count -> %3d syncing -> %5d purne blocks -> %3d", len(b.Index), b.LocalBlocks, b.PruneBlocks)
}

func (b *BTCNode) DeleteIndex(key string) error {
	lock.Lock()
	delete(b.Index, key)
	lock.Unlock()
	err := b.db.Delete([]byte("index_"+key), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) DeleteTxs(key string) error {
	lock.Lock()
	delete(b.Txs, key)
	lock.Unlock()
	err := b.db.Delete([]byte("txs_"+key), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) PutIndex(addr string, confirms int64, txID string) error {
	lock.RLock()
	meta := b.Index[addr]
	lock.RUnlock()
	txs := []string{}
	count := 0
	height := confirms
	if meta != nil {
		txs = meta.Txs
		count = meta.Count + 1
		if height > meta.Height {
			height = meta.Height
		}
	}
	txs = append(txs, txID)
	newMeta := &Meta{
		Height: height,
		Count:  count,
		Txs:    txs,
	}
	m, err := json.Marshal(newMeta)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	err = b.db.Put([]byte("index_"+addr), []byte(m), nil)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	lock.Lock()
	b.Index[addr] = newMeta
	lock.Unlock()
	//log.Info("put index -> ", addr)
	return nil
}

func (b *BTCNode) PutTxs(key string, tx *Tx) error {
	t, err := json.Marshal(tx)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	err = b.db.Put([]byte("txs_"+key), []byte(t), nil)
	if err != nil {
		fmt.Printf("Error: %s", err)
		return err
	}
	lock.Lock()
	b.Txs[tx.Txid] = tx
	lock.Unlock()
	//log.Info("put tx -> ", tx.Txid)
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
