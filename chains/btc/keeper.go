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
	interval   = 30 * time.Second
	loadBlocks = 3
)

type Keeper struct {
	mu         *sync.RWMutex
	client     *Client
	ticker     *time.Ticker
	tesnet     bool
	db         *common.Db
	mempoolTxs map[string]MempoolTx
	topHeight  int64
}

type MempoolTx struct {
	common.Transaction
	end time.Time
}

func NewKeeper(url string, isTestnet bool, dirPath string, pruneTime int64) *Keeper {
	c, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	newDB := common.NewDB()
	err = newDB.Start(dirPath, pruneTime)
	if err != nil {
		log.Fatal(err)
	}
	k := &Keeper{
		mu:         new(sync.RWMutex),
		client:     c,
		tesnet:     isTestnet,
		db:         newDB,
		mempoolTxs: make(map[string]MempoolTx),
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
	mempoolTxs := common.Txs{}
	k.mu.Lock()
	for _, tx := range k.mempoolTxs {
		mempoolTxs = append(mempoolTxs, tx.Transaction)
	}
	k.mu.Unlock()
	k.mu.RLock()
	w.WriteJson(common.TxResponse{
		LatestHeight:  k.topHeight,
		InTxsMempool:  mempoolTxs.Sort().ReceiveMempool(watch).Page(pageNum, limitNum),
		OutTxsMempool: mempoolTxs.Sort().SendMempool(watch).Page(pageNum, limitNum),
		InTxs:         inTxs.Sort().Page(pageNum, limitNum),
		OutTxs:        outTxs.Sort().Page(pageNum, limitNum),
		Response: common.Response{
			Msg:    "",
			Result: true,
		},
	})
	k.mu.RUnlock()
}

func (k *Keeper) GetTx(w rest.ResponseWriter, r *rest.Request) {
	txHash := r.URL.Query().Get("tx_hash")
	txs := []common.Transaction{}
	for i := 0; i <= 10000; i++ {
		key := fmt.Sprintf("%s;%d;", strings.ToLower(txHash), i)
		// Check mempool
		k.mu.RLock()
		if _, ok := k.mempoolTxs[key]; ok {
			txs = append(txs, k.mempoolTxs[key].Transaction)
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
	k.ticker = time.NewTicker(interval)
	k.processKeep()
	k.StartNode()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				k.processKeep()
			}
		}
	}()
}

func (k *Keeper) processKeep() {
	topHeight, rawTxs := k.client.GetBlockTxs(k.tesnet, loadBlocks)
	txs := []common.Transaction{}
	for _, tx := range rawTxs {
		// if index == 0 {
		// 	// coinbase tx
		// 	continue
		// }
		isNew := false
		for i := range tx.Vout {
			// coinbse tx
			key := fmt.Sprintf("%s;%d;", tx.Txid, i)
			_, err := k.db.GetTx(key)
			if err != nil {
				isNew = true
			}
		}
		if isNew {
			commonTxs := k.client.TxtoCommonTx(tx, k.tesnet)
			for _, comTx := range commonTxs {
				txs = append(txs, comTx)
			}
		}
	}
	k.StoreTxs(txs)
	k.mu.Lock()
	k.topHeight = topHeight
	log.Infof("BTC txs scanning done -> txs: %d mempool: %d", len(txs), len(k.mempoolTxs))
	k.mu.Unlock()
	k.UpdateMempool()
}

func (k *Keeper) StoreTxs(txs []common.Transaction) {
	for _, tx := range txs {
		_, err := k.db.GetTx(tx.Serialize())
		if err == nil {
			continue
		}
		k.db.StoreTx(tx.Serialize(), &tx)
		k.db.StoreIdx(tx.Serialize(), &tx, true)
		k.db.StoreIdx(tx.Serialize(), &tx, false)
		k.RemoveMempoolTx(&tx)
	}
}

func (k *Keeper) UpdateMempool() {
	keys := []string{}
	k.mu.RLock()
	for _, memTx := range k.mempoolTxs {
		if memTx.end.Unix() < time.Now().Unix() {
			keys = append(keys, memTx.Serialize())
		}
	}
	k.mu.RUnlock()
	for _, key := range keys {
		k.mu.Lock()
		delete(k.mempoolTxs, key)
		k.mu.Unlock()
	}
}

func (k *Keeper) RemoveMempoolTx(tx *common.Transaction) {
	go func() {
		k.mu.Lock()
		if _, ok := k.mempoolTxs[tx.Serialize()]; ok {
			// Even if the tx is mined, tx is still exist on the mempool until prune time expired.
			//time.Sleep(30 * time.Minute)
			delete(k.mempoolTxs, tx.Serialize())
		}
		k.mu.Unlock()
	}()
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
	hash, err := k.client.SendRawTransaction(msgTx, false)
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
	txChan := make(chan *types.Tx)
	BChan := make(chan *types.Block)
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
			tx := <-txChan
			// Set now time
			go func() {
				time.Sleep(5 * time.Second)
				commonTxs := k.client.TxtoCommonTx(*tx, k.tesnet)
				if len(commonTxs) == 0 {
					return
				}
				for _, tx := range commonTxs {
					k.mu.Lock()
					k.mempoolTxs[tx.Serialize()] = MempoolTx{tx, time.Now().Add(30 * time.Minute)}
					k.mu.Unlock()
				}
			}()
		}
	}()
}
