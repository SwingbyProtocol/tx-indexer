package btc

import (
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/node"
	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/btcsuite/btcutil"
	log "github.com/sirupsen/logrus"
)

const (
	interval   = 30 * time.Second
	loadBlocks = 10
)

type Keeper struct {
	mu         *sync.RWMutex
	client     *Client
	ticker     *time.Ticker
	watchAddr  btcutil.Address
	tesnet     bool
	db         *common.Db
	mempoolTxs map[string]common.Transaction
	topHeight  int64
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
		mempoolTxs: make(map[string]common.Transaction),
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
		mempoolTxs = append(mempoolTxs, tx)
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
	topHeight, txs := k.client.GetBlockTxs(true, loadBlocks)
	k.StoreTxs(txs)
	k.mu.Lock()
	k.topHeight = topHeight
	log.Infof("BTC txs scanning done -> txs: %d mempool: %d", len(txs), len(k.mempoolTxs))
	k.mu.Unlock()
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
		k.mu.Lock()
		if _, ok := k.mempoolTxs[tx.Serialize()]; ok {
			delete(k.mempoolTxs, tx.Serialize())
		}
		k.mu.Unlock()
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
				time.Sleep(7 * time.Second)
				commonTxs := k.client.TxtoCommonTx(*tx, k.tesnet)
				if len(commonTxs) == 0 {
					log.Warn("len(commonTxs) == 0. Something wrong on the pending tx")
				}
				for _, tx := range commonTxs {
					k.mu.Lock()
					k.mempoolTxs[tx.Serialize()] = tx
					k.mu.Unlock()
				}
			}()
		}
	}()
}
