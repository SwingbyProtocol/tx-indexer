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
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	log "github.com/sirupsen/logrus"
)

const (
	interval = 30 * time.Second
)

type Keeper struct {
	mu           *sync.RWMutex
	client       *Client
	ticker       *time.Ticker
	watchAddr    btcutil.Address
	tesnet       bool
	txs          map[string]common.Transaction
	latestHeight int64
	isScanEnd    bool
}

func NewKeeper(url string, isTestnet bool) *Keeper {
	c, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	k := &Keeper{
		mu:        new(sync.RWMutex),
		client:    c,
		tesnet:    isTestnet,
		txs:       make(map[string]common.Transaction),
		isScanEnd: false,
	}
	return k
}

func (k *Keeper) StartReScanAddr(timestamp int64) error {
	k.mu.RLock()
	addr := k.watchAddr.EncodeAddress()
	k.mu.RUnlock()
	// set rescan

	opts := []btcjson.ImportMultiRequest{{
		ScriptPubKey: btcjson.ImportMultiRequestScriptPubKey{Address: addr},
		Timestamp:    &timestamp,
		WatchOnly:    true,
	}}
	res := k.client.ImportMultiRescanAsync(opts, true)
	// wait for the async importaddress result; log out any error that comes through
	go func(res rpcclient.FutureImportAddressResult) {
		log.Infof("bitcoind ImportMulti rescan begin: %s", addr)
		k.mu.Lock()
		k.isScanEnd = false
		k.mu.Unlock()
		if err := res.Receive(); err != nil {
			log.Infof("bitcoind ImportMulti returned an error: %s", err)
			return
		}
		k.mu.Lock()
		k.isScanEnd = true
		k.mu.Unlock()
		log.Infof("bitcoind ImportMulti rescan done: %s", addr)

	}(res)
	return nil
}

func (k *Keeper) GetAddr() btcutil.Address {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.watchAddr
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
	//log.Info(k.Txs)
	txs := common.Txs{}
	k.mu.RLock()
	isScan := k.isScanEnd
	for _, tx := range k.txs {
		txs = append(txs, tx)
	}
	k.mu.RUnlock()
	if !isScan {
		res := common.Response{
			Result: false,
			Msg:    "re-scanning",
		}
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	rangeTxs := txs.GetRangeTxs(fromNum, toNum).Sort()
	memPoolTxs := txs.GetRangeTxs(0, 0).Sort()

	inTxsMemPool := memPoolTxs.ReceiveMempool(watch).Page(pageNum, limitNum)
	outTxsMemPool := memPoolTxs.SendMempool(watch).Page(pageNum, limitNum)
	inTxs := rangeTxs.Receive(watch).Page(pageNum, limitNum)
	outTxs := rangeTxs.Send(watch).Page(pageNum, limitNum)

	w.WriteJson(common.TxResponse{
		InTxsMempool:  inTxsMemPool,
		OutTxsMempool: outTxsMemPool,
		InTxs:         inTxs,
		OutTxs:        outTxs,
		Response: common.Response{
			Msg:    "",
			Result: true,
		},
	})
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
				k.UpdateTxs()
			}
		}
	}()
}

func (k *Keeper) UpdateTxs() {
	deleteList := []string{}
	k.mu.RLock()
	txs := k.txs
	k.mu.RUnlock()
	for _, tx := range txs {
		if tx.Timestamp.Add(48*time.Hour).Unix() < time.Now().Unix() {
			deleteList = append(deleteList, tx.TxID)
			continue
		}
	}
	for _, txID := range deleteList {
		k.mu.Lock()
		delete(k.txs, txID)
		k.mu.Unlock()
	}
}

func (k *Keeper) processKeep() {
	depth := 16
	k.mu.RLock()
	isScan := k.isScanEnd
	k.mu.RUnlock()
	if !isScan {
		depth = 290 // about 6 blocks / hour * 48
	}
	latestHeight, txs := k.client.GetBlockTxs(true, depth)
	k.mu.Lock()
	for _, tx := range txs {
		k.txs[tx.Serialize()] = tx
	}
	for _, tx := range k.txs {
		if tx.Height == 0 {
			continue
		}
		tx.Confirmations = latestHeight - tx.Height + 1
		k.txs[tx.Serialize()] = tx
	}
	if !k.isScanEnd {
		k.isScanEnd = true
	}
	log.Infof("BTC txs scanning done -> loadTxs: %d", len(k.txs))
	k.mu.Unlock()
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
			go func() {
				time.Sleep(5 * time.Second)
				commonTxs := k.client.TxtoCommonTx(*tx, k.tesnet)
				for _, comTx := range commonTxs {
					k.mu.Lock()
					k.txs[comTx.Serialize()] = comTx
					k.mu.Unlock()
				}
			}()
		}
	}()
}
