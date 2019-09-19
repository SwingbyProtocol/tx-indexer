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
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	lock     = sync.RWMutex{}
	taskList = []Task{}
)

type Node struct {
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

func NewNode(uri string, pruneBlocks int64) *Node {
	db, err := leveldb.OpenFile("./db", nil)
	if err != nil {
		log.Fatal(err)
	}
	node := &Node{
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

func (node *Node) Start() {
	err := loadData(node)
	if err != nil {
		log.Info(err)
	}
	Loop(node.getTxs, 10000)
	Loop(node.work, 10)
}

func (node *Node) getTxs() {
	tasks, err := checkMemPool(node.Resolver, node.URI)
	if err != nil {
		log.Info(err)
		return
	}
	if node.Status != "stop" {
		return
	}
	node.Status = "start"
	for _, task := range tasks {
		lock.Lock()
		if node.Pool[task.Txid] != true {
			taskList = append(taskList, task)
		}
		lock.Unlock()
	}
	go node.removePools(tasks)
	go node.showIndex()
	go checkBlock(node.Resolver, node.db, node.URI)
	lock.RLock()
	log.Infof("pushed -> %7d spent -> %7d pool -> %7d index -> %7d", len(taskList), len(node.Spent), len(node.Pool), len(node.Index))
	lock.RUnlock()
}

func (node *Node) removePools(tasks []Task) {
	lock.Lock()
	for key := range node.Pool {
		isMatch := false
		for _, task := range tasks {
			txID := task.Txid
			if key == txID {
				isMatch = true
			}
		}
		if isMatch == false {
			delete(node.Pool, key)
			go removePool(node.db, key)
		}
	}
	lock.Unlock()
}

func (node *Node) work() {
	lock.Lock()
	if len(taskList) == 0 {
		node.Status = "stop"
		lock.Unlock()
		return
	}
	task := taskList[0]
	taskList = taskList[1:]
	lock.Unlock()
	txID := task.Txid
	receiveTime := task.ReceivedTime
	tx, err := checkTx(node.Resolver, node.URI, txID)
	if err != nil {
		task.Errors++
		log.Infof("%s count -> %d", err, task.Errors)
		if task.Errors < 12 {
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
		if node.Spent[key] == "" {
			node.Spent[key] = tx.Txid
		} else {
			if splitCheck(node.Spent[key], tx.Txid) {
				continue
			}
			node.Spent[key] = node.Spent[key] + "_" + tx.Txid
		}
		go storeSpent(node.db, key, node.Spent[key])
	}
	for _, vout := range tx.Vout {
		for _, addr := range vout.ScriptPubkey.Addresses {
			if node.Index[addr] == "" {
				node.Index[addr] = tx.Txid
			} else {
				if splitCheck(node.Index[addr], tx.Txid) {
					continue
				}
				node.Index[addr] = node.Index[addr] + "_" + tx.Txid
			}
			go storeIndex(node.db, addr, node.Index[addr])
		}
	}
	node.Pool[txID] = true
	lock.Unlock()
	go storePool(node.db, txID)
	go storeTx(node.db, txID, tx)
}

func (node *Node) showIndex() {
	lock.RLock()
	for addr, value := range node.Index {
		txIDs := strings.Split(value, "_")
		if len(txIDs) > 6*int(node.PruneBlocks) {
			log.Infof("counts -> %12d addr -> %s ", len(txIDs), addr)
		}
	}
	lock.RUnlock()
}

func (node *Node) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	address := r.PathParam("address")
	sortFlag := r.FormValue("sort")
	pageFlag := r.FormValue("page")
	spentFlag := r.FormValue("type")
	lock.RLock()
	if node.Index[address] == "" {
		lock.RUnlock()
		res500(w, r)
		return
	}
	txIDs := strings.Split(node.Index[address], "_")
	lock.RUnlock()
	txRes := []*Tx{}
	// get txs for received
	txs := loadTxs(node.db, txIDs)
	if spentFlag == "send" {
		// get sending txs from txs
		spents := node.getSpending(address, txs)
		spentTxs := loadTxs(node.db, spents)
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
			vout.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
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
			if node.Spent[key] == "" {
				lock.RUnlock()
				continue
			}
			txIDs := strings.Split(node.Spent[key], "_")
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
		}
		txRes = txRes[p:limit]
	}
	w.WriteHeader(http.StatusOK)
	w.WriteJson(txRes)
}

func (b *Node) getSpending(from string, txs []*Tx) []string {
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

func checkBlock(r *resolver.Resolver, db *leveldb.DB, uri string) error {
	info := ChainInfo{}
	err := r.GetRequest(uri, "/rest/chaininfo.json", &info)
	if err != nil {
		return err
	}
	block := Block{}
	err = r.GetRequest(uri, "/rest/block/notxdetails/"+info.BestBlockHash+".json", &block)
	if err != nil {
		return err
	}
	log.Info("get block -> ", block.Height)
	txs := loadTxs(db, block.Txs)
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
		go storeTx(db, tx.Txid, tx)
	}
	return nil
}

func checkTx(r *resolver.Resolver, uri string, txID string) (*Tx, error) {
	tx := Tx{}
	err := r.GetRequest(uri, "/rest/tx/"+txID+".json", &tx)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func checkMemPool(r *resolver.Resolver, uri string) ([]Task, error) {
	res := make(map[string]PoolTx)
	err := r.GetRequest(uri, "/rest/mempool/contents.json", &res)
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

func res500(w rest.ResponseWriter, r *rest.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	res := []string{}
	w.WriteJson(res)
}
