package btc

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	lock     = sync.RWMutex{}
	taskList = []Task{}
)

type BTCNode struct {
	Index       map[string]string
	Spent       map[string]string
	Pool        map[string]bool
	Updates     map[string]int64
	PruneBlocks int64
	LocalBlocks int64
	BestBlocks  int64
	Resolver    *resolver.Resolver
	URI         string
	db          *leveldb.DB
	Status      string
}

type Task struct {
	Txid         string
	ReceivedTime int64
	Errors       int
}

type PoolTx struct {
	Height int64 `json:"height"`
	Time   int64 `json:"time"`
}

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

type Block struct {
	Hash              string   `json:"hash"`
	Confirmations     int64    `json:"confirmations"`
	Height            int64    `json:"height"`
	NTx               int64    `json:"nTx"`
	Txs               []string `json:"tx"`
	Time              int64    `json:"time"`
	Mediantime        int64    `json:"mediantime"`
	Previousblockhash string   `json:"previousblockhash"`
}

func NewBTCNode(uri string, pruneBlocks int64) *BTCNode {
	db, err := leveldb.OpenFile("./db", nil)
	if err != nil {
		log.Fatal(err)
	}
	node := &BTCNode{
		Index:       make(map[string]string),
		Spent:       make(map[string]string),
		Pool:        make(map[string]bool),
		Updates:     make(map[string]int64),
		URI:         uri,
		PruneBlocks: pruneBlocks,
		Resolver:    resolver.NewResolver(),
		db:          db,
		Status:      "init",
	}
	return node
}

func (b *BTCNode) Start() {
	err := b.loadData()
	if err != nil {
		log.Info(err)
	}
	b.Loop(b.GetNewTxs, 10000)
	b.Loop(b.Work, 20)
}

func (b *BTCNode) Loop(f func(), t time.Duration) {
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

func (b *BTCNode) GetNewTxs() {
	tasks, err := b.checkMemPool()
	if err != nil {
		log.Info(err)
		return
	}
	if b.Status != "stop" {
		return
	}
	b.Status = "start"
	for _, task := range tasks {
		lock.Lock()
		if b.Pool[task.Txid] != true {
			taskList = append(taskList, task)
		}
		lock.Unlock()
	}
	go b.removePools(tasks)
	go b.checkBlock()
	go b.showIndex()
	lock.RLock()
	log.Infof("pushed -> %7d spent -> %7d pool -> %7d index -> %7d", len(taskList), len(b.Spent), len(b.Pool), len(b.Index))
	lock.RUnlock()
}

func (b *BTCNode) removePools(tasks []Task) {
	lock.Lock()
	for key := range b.Pool {
		isMatch := false
		for _, task := range tasks {
			txID := task.Txid
			if key == txID {
				isMatch = true
			}
		}
		if isMatch == false {
			delete(b.Pool, key)
			go b.removePool(key)
		}
	}
	lock.Unlock()
}

func (b *BTCNode) checkBlock() error {
	info := ChainInfo{}
	err := b.Resolver.GetRequest(b.URI, "/rest/chaininfo.json", &info)
	if err != nil {
		return err
	}
	block := Block{}
	err = b.Resolver.GetRequest(b.URI, "/rest/block/notxdetails/"+info.BestBlockHash+".json", &block)
	if err != nil {
		return err
	}
	log.Info("get block -> ", block.Height)
	txs := b.LoadTxs(block.Txs)
	for _, tx := range txs {
		if tx.Txid == "" {
			continue
		}
		if tx.Confirms != 0 {
			continue
		}
		tx.Confirms = block.Height
		tx.MinedTime = block.Time
		tx.Mediantime = block.Mediantime
		go b.storeTx(tx.Txid, tx)
	}
	return nil
}

func (b *BTCNode) Work() {
	lock.Lock()
	if len(taskList) == 0 {
		b.Status = "stop"
		lock.Unlock()
		return
	}
	task := taskList[0]
	taskList = taskList[1:]
	lock.Unlock()
	txID := task.Txid
	receiveTime := task.ReceivedTime
	tx, err := b.checkTx(txID)
	if err != nil {
		task.Errors++
		log.Infof("%s count -> %d", err, task.Errors)
		if task.Errors < 30 {
			go func() {
				time.Sleep(10 * time.Second)
				taskList = append(taskList, task)
			}()
		}
		return
	}
	tx.ReceivedTime = receiveTime
	lock.Lock()
	for _, vin := range tx.Vin {
		key := vin.Txid + "_" + strconv.Itoa(vin.Vout)
		if b.Spent[key] == "" {
			b.Spent[key] = tx.Txid
		} else {
			if splitCheck(b.Spent[key], tx.Txid) {
				continue
			}
			b.Spent[key] = b.Spent[key] + "_" + tx.Txid
		}
		go b.storeSpent(key, b.Spent[key])
	}
	for _, vout := range tx.Vout {
		for _, addr := range vout.ScriptPubkey.Addresses {
			if b.Index[addr] == "" {
				b.Index[addr] = tx.Txid
			} else {
				if splitCheck(b.Index[addr], tx.Txid) {
					continue
				}
				b.Index[addr] = b.Index[addr] + "_" + tx.Txid
			}
			go b.storeIndex(addr, b.Index[addr])
		}
	}
	b.Pool[txID] = true
	lock.Unlock()
	go b.storePool(txID)
	go b.storeTx(txID, tx)
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

func (b *BTCNode) checkTx(txID string) (*Tx, error) {
	tx := Tx{}
	err := b.Resolver.GetRequest(b.URI, "/rest/tx/"+txID+".json", &tx)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (b *BTCNode) checkMemPool() ([]Task, error) {
	res := make(map[string]PoolTx)
	err := b.Resolver.GetRequest(b.URI, "/rest/mempool/contents.json", &res)
	if err != nil {
		return nil, err
	}
	txs := []Task{}
	height := int64(0)
	for _, tx := range res {
		if tx.Height > height {
			height = tx.Height
		}
	}
	for id, tx := range res {
		if tx.Height == height {
			neTask := Task{id, tx.Time, 0}
			txs = append(txs, neTask)
		}
	}
	return txs, nil
}

func (b *BTCNode) showIndex() {
	lock.RLock()
	for addr, value := range b.Index {
		txIDs := strings.Split(value, "_")
		if len(txIDs) > 6*int(b.PruneBlocks) {
			log.Infof("counts -> %12d addr -> %s ", len(txIDs), addr)
		}
	}
	lock.RUnlock()
}

func (b *BTCNode) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	sortFlag := r.FormValue("sort")
	pageFlag := r.FormValue("page")
	spentFlag := r.FormValue("type")
	lock.RLock()
	if b.Index[address] == "" {
		lock.RUnlock()
		res500(w, r)
		return
	}
	txIDs := strings.Split(b.Index[address], "_")
	lock.RUnlock()
	txRes := []*Tx{}
	// get txs for received
	txs := b.LoadTxs(txIDs)
	if spentFlag == "send" {
		// get sending txs from txs
		spents := b.getSpending(address, txs)
		spentTxs := b.LoadTxs(spents)
		for _, spent := range spentTxs {
			txRes = append(txRes, spent)
		}
	} else {
		for _, tx := range txs {
			txRes = append(txRes, tx)
		}
	}
	for _, tx := range txRes {
		for _, vout := range tx.Vout {
			vout.Txs = []string{}
			isMatchAddr := false
			for _, addr := range vout.ScriptPubkey.Addresses {
				if addr == address {
					isMatchAddr = true
				}
			}
			if isMatchAddr == false {
				continue
			}
			key := tx.Txid + "_" + strconv.Itoa(vout.N)
			lock.RLock()
			if b.Spent[key] == "" {
				lock.RUnlock()
				continue
			}
			txIDs := strings.Split(b.Spent[key], "_")
			lock.RUnlock()
			vout.Spent = true
			for _, txID := range txIDs {
				vout.Txs = append(vout.Txs, txID)
			}
		}
		isSpent := false
		for _, vout := range tx.Vout {
			if vout.Spent == true {
				isSpent = true
			}
		}
		if isSpent == true {
			tx.ReceivedTime--
		}
	}

	if sortFlag == "asc" {
		sort.SliceStable(txRes, func(i, j int) bool {
			return txRes[i].ReceivedTime < txRes[j].ReceivedTime
		})
	} else {
		sort.SliceStable(txRes, func(i, j int) bool {
			return txRes[i].ReceivedTime > txRes[j].ReceivedTime
		})
	}
	pageNum, err := strconv.Atoi(pageFlag)
	if err != nil {
		pageNum = 0
	}
	if len(txRes) >= 100 {
		p := pageNum * 100
		limit := p + 100
		if len(txRes) < limit {
			p = 100 * (len(txRes) / 100)
			limit = len(txRes)
			//log.Info(p, " ", limit, " ", len(txRes))
		}
		txRes = txRes[p:limit]
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txRes)
}

func (b *BTCNode) getSpending(from string, txs []*Tx) []string {
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
			key := tx.Txid + "_" + strconv.Itoa(vout.N)
			lock.RLock()
			if b.Spent[key] == "" {
				lock.RUnlock()
				continue
			}
			txIDs := strings.Split(b.Spent[key], "_")
			for _, txID := range txIDs {
				isAdd := false
				for _, spent := range spents {
					if spent == txID {
						isAdd = true
					}
				}
				if isAdd == false {
					spents = append(spents, txID)
				}
			}
			lock.RUnlock()
		}
	}
	return spents
}

func (b *BTCNode) LoadTxs(txIDs []string) []*Tx {
	txRes := []*Tx{}
	c := make(chan Tx)
	for _, txID := range txIDs {
		go b.LoadTx("tx_"+txID, c)
	}
	for i := 0; i < len(txIDs); i++ {
		tx := <-c
		txRes = append(txRes, &tx)
	}
	return txRes
}

func (b *BTCNode) LoadTx(key string, c chan Tx) {
	tx := Tx{}
	value, err := b.db.Get([]byte(key), nil)
	if err != nil {
		c <- tx
		return
	}
	json.Unmarshal(value, &tx)
	c <- tx
}

func res500(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	res := []string{}
	w.WriteJson(res)
}
