package btc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/node"
	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

const (
	loadBlockInterval   = 24 * time.Second
	loadMempoolInterval = 3 * time.Second
	loadBlocks          = 1
)

type Keeper struct {
	mu        *sync.RWMutex
	c1        *Client
	c2        *Client
	ticker    *time.Ticker
	tesnet    bool
	db        *common.Db
	pendings  map[string]int
	isScan    bool
	topHeight int64
}

type MempoolTx struct {
	common.Transaction
	end time.Time
}

func NewKeeper(url string, isTestnet bool, dirPath string, pruneTime int64) *Keeper {
	c1, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	c2, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	newDB := common.NewDB()
	err = newDB.Start(dirPath, pruneTime)
	if err != nil {
		log.Fatal(err)
	}
	k := &Keeper{
		mu:       new(sync.RWMutex),
		c1:       c1,
		c2:       c2,
		tesnet:   isTestnet,
		db:       newDB,
		pendings: make(map[string]int),
	}
	return k
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	watch := r.URL.Query().Get("watch")
	from := r.URL.Query().Get("height_from")
	fromNum, _ := strconv.Atoi(from)
	to := r.URL.Query().Get("height_to")
	toNum, _ := strconv.Atoi(to)
	page := r.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(page)
	limit := r.URL.Query().Get("limit")
	limitNum, _ := strconv.Atoi(limit)
	if toNum == 0 {
		toNum = 100000000
	}
	inIdxs, _ := k.db.GetIdxs(watch, true)
	inIdxs = common.Idxs(inIdxs).GetRangeTxs(fromNum, toNum)
	inTxs := common.Txs{}
	for _, idx := range inIdxs {
		tx, err := k.db.GetTx(idx.ID)
		if err != nil {
			continue
		}
		inTxs = append(inTxs, *tx)
	}
	outIdxs, _ := k.db.GetIdxs(watch, false)
	outIdxs = common.Idxs(outIdxs).GetRangeTxs(fromNum, toNum)
	outTxs := common.Txs{}
	for _, idx := range outIdxs {
		tx, err := k.db.GetTx(idx.ID)
		if err != nil {
			continue
		}
		outTxs = append(outTxs, *tx)
	}
	mempoolTxs, _ := k.db.GetMempoolTxs(watch)
	k.mu.RLock()
	topHeight := k.topHeight
	k.mu.RUnlock()
	w.WriteJson(common.TxResponse{
		LatestHeight:  topHeight,
		InTxsMempool:  common.Txs(mempoolTxs).Sort().ReceiveMempool(watch).Page(pageNum, limitNum),
		OutTxsMempool: common.Txs(mempoolTxs).Sort().SendMempool(watch).Page(pageNum, limitNum),
		InTxs:         inTxs.Sort().Page(pageNum, limitNum),
		OutTxs:        outTxs.Sort().Page(pageNum, limitNum),
		Response: common.Response{
			Msg:    "",
			Result: true,
		},
	})
}

func (k *Keeper) GetTx(w rest.ResponseWriter, r *rest.Request) {
	txHash := r.URL.Query().Get("tx_hash")
	txs := []common.Transaction{}
	for i := 0; i <= 10000; i++ {
		key := fmt.Sprintf("%s;%d;", strings.ToLower(txHash), i)
		// Check mempool
		k.mu.RLock()
		if _, ok := k.pendings[key]; ok {
			//txs = append(txs, k.mempoolTxs[key].Transaction)
			k.mu.RUnlock()
			continue
		}
		k.mu.RUnlock()
		tx, err := k.db.GetTx(key)
		if err != nil {
			w.WriteJson(txs)
			return
		}
		txs = append(txs, *tx)
	}
	w.WriteJson(txs)
}

func (k *Keeper) Start() {
	k.ticker = time.NewTicker(loadBlockInterval)
	k.processKeep()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				k.processKeep()
			}
		}
	}()
	mempoolTicker := time.NewTicker(loadMempoolInterval)
	go func() {
		for {
			select {
			case <-mempoolTicker.C:
				k.UpdateMemPoolTxs()
			}
		}
	}()
	k.StartNode()
}

func (k *Keeper) processKeep() {
	if k.isScan {
		return
	}
	k.isScan = true
	topHeight, rawTxs := k.c1.GetBlockTxs(k.tesnet, loadBlocks)
	txs := []common.Transaction{}
	for _, tx := range rawTxs {
		comTxs := k.txToComTxs(tx)
		for _, tx := range comTxs {
			txs = append(txs, tx)
		}
	}
	log.Infof("BTC txs scanning done -> txs: %d", len(txs))
	for _, tx := range txs {
		k.db.StoreTx(tx.Serialize(), &tx)
		k.db.StoreIdx(tx.Serialize(), &tx, true)
		k.db.StoreIdx(tx.Serialize(), &tx, false)
	}
	k.isScan = false
	k.mu.Lock()
	k.topHeight = topHeight
	k.mu.Unlock()
}

func (k *Keeper) txToComTxs(tx *types.Tx) []common.Transaction {
	isNew := false
	// Check tx in the disk
	for i := range tx.Vout {
		key := fmt.Sprintf("%s;%d;", tx.Txid, i)
		_, err := k.db.GetTx(key)
		if err != nil {
			isNew = true
		}
	}
	txs := []common.Transaction{}
	if isNew {
		commonTxs, err := k.c1.TxtoCommonTx(tx, k.tesnet)
		if err != nil {
			log.Info(err)
			return txs
		}
		for _, comTx := range commonTxs {
			txs = append(txs, comTx)
		}
	}
	return txs
}

func (k *Keeper) GetPendings() map[string]int {
	pendings := make(map[string]int)
	k.mu.RLock()
	defer k.mu.RUnlock()
	for txid, count := range k.pendings {
		pendings[txid] = count
	}
	return pendings
}

func (k *Keeper) UpdateMemPoolTxs() {
	pendings := k.GetPendings()
	if len(pendings) == 0 {
		return
	}
	log.Info("pendings -> ", len(pendings))
	for txid, count := range pendings {
		go k.UpdateTx(txid, count)
	}
}

func (k *Keeper) UpdateTx(txid string, count int) {
	if count >= 20 {
		k.mu.Lock()
		delete(k.pendings, txid)
		k.mu.Unlock()
		return
	}
	tx, err := k.c2.GetTxByTxID(txid, k.tesnet)
	if err != nil {
		k.mu.Lock()
		k.pendings[txid]++
		k.mu.Unlock()
		return
	}
	commonTxs, err := k.c2.TxtoCommonTx(tx, k.tesnet)
	if err != nil {
		k.mu.Lock()
		delete(k.pendings, txid)
		k.mu.Unlock()
		return
	}
	if len(commonTxs) == 0 {
		k.mu.Lock()
		k.pendings[txid]++
		k.mu.Unlock()
		return
	}
	k.mu.Lock()
	delete(k.pendings, txid)
	k.mu.Unlock()
	for _, commTx := range commonTxs {
		k.db.AddMempoolTxs(commTx.From, commTx)
		k.db.AddMempoolTxs(commTx.To, commTx)
	}
}

func (k *Keeper) BroadcastTx(w rest.ResponseWriter, r *rest.Request) {
	req := common.BroadcastParams{}
	res := common.BroadcastResponse{}
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	msgTx, err := utils.DecodeToTx(req.HEX)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	hash, err := k.c2.SendRawTransaction(msgTx, false)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	res.TxHash = hash.String()
	res.Result = true
	res.Msg = "success"
	w.WriteJson(res)
}

func (k *Keeper) StartNode() {
	txChan := make(chan *types.Tx, 10000)
	BChan := make(chan *types.Block, 3)
	nodeConfig := &node.NodeConfig{
		IsTestnet:        k.tesnet,
		TargetOutbound:   25,
		UserAgentName:    "Tx-indexer",
		UserAgentVersion: "1.0.0",
		TxChan:           txChan,
		BChan:            BChan,
	}
	// Node initialize
	n := node.NewNode(nodeConfig)
	// Node Start
	go n.Start()
	go func() {
		for {
			select {
			case tx := <-txChan:
				k.mu.Lock()
				k.pendings[tx.Txid] = 1
				k.mu.Unlock()
			}
		}
	}()
}
