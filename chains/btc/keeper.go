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
	"github.com/btcsuite/btcd/chaincfg"
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
	accessToken  string
	txs          map[string]common.Transaction
	latestHeight int64
	isScanEnd    bool
}

type State struct {
	common.Response
	InTxsMempool  []common.Transaction `json:"inTxsMempool"`
	InTxs         []common.Transaction `json:"inTxs"`
	OutTxsMempool []common.Transaction `json:"outTxsMempool"`
	OutTxs        []common.Transaction `json:"outTxs"`
}

func NewKeeper(url string, isTestnet bool, accessToken string) *Keeper {
	c, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	k := &Keeper{
		mu:          new(sync.RWMutex),
		client:      c,
		tesnet:      isTestnet,
		accessToken: accessToken,
		txs:         make(map[string]common.Transaction),
		isScanEnd:   false,
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

func (k *Keeper) SetWatchAddr(addr string, rescan bool, timestamp int64) error {
	net := &chaincfg.MainNetParams
	if k.tesnet {
		net = &chaincfg.TestNet3Params
	}
	address, err := btcutil.DecodeAddress(addr, net)
	if err != nil {
		return err
	}
	k.mu.Lock()
	k.watchAddr = address
	k.isScanEnd = true
	log.Infof("set btc watch address -> %s rescan: %t timestamp: %d", k.watchAddr.String(), rescan, timestamp)
	k.mu.Unlock()
	if rescan {
		k.StartReScanAddr(timestamp)
	}
	return nil
}

func (k *Keeper) GetAddr() btcutil.Address {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.watchAddr
}

func (k *Keeper) SetConfig(w rest.ResponseWriter, r *rest.Request) {
	req := common.ConfigParams{}
	res := common.Response{}
	err := r.DecodeJsonPayload(&req)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	if k.accessToken != req.AccessToken {
		res.Msg = "AccessToken is not valid"
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	err = k.SetWatchAddr(req.Address, req.IsRescan, req.Timestamp)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	res.Result = true
	res.Msg = "success"
	w.WriteJson(res)
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	k.mu.RLock()
	defer k.mu.RUnlock()
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
	k.mu.RLock()
	defer k.mu.RUnlock()
	if !k.isScanEnd {
		res := common.Response{
			Result: false,
			Msg:    "re-scanning",
		}
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	//log.Info(k.Txs)
	txs := common.Txs{}
	for _, tx := range k.txs {
		txs = append(txs, tx)
	}
	rangeTxs := txs.GetRangeTxs(fromNum, toNum).Sort()
	memPoolTxs := txs.GetRangeTxs(0, 0).Sort()

	inTxsMemPool := memPoolTxs.ReceiveMempool(watch).Page(pageNum, limitNum)
	outTxsMemPool := memPoolTxs.SendMempool(watch).Page(pageNum, limitNum)
	inTxs := rangeTxs.Receive(watch).Page(pageNum, limitNum)
	outTxs := rangeTxs.Send(watch).Page(pageNum, limitNum)

	w.WriteJson(State{
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
	targetTime := time.Now().Add(-48 * time.Hour)
	deleteList := []string{}
	k.mu.Lock()
	for _, tx := range k.txs {
		if tx.Timestamp.Unix() < targetTime.Unix() {
			deleteList = append(deleteList, tx.TxID)
			continue
		}
	}
	for _, txID := range deleteList {
		delete(k.txs, txID)
	}
	k.mu.Unlock()
}

func (k *Keeper) processKeep() {
	depth := 16
	k.mu.RLock()
	if !k.isScanEnd {
		depth = 260
	}
	k.mu.RUnlock()
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
	n.Start()
	go func() {
		for {
			tx := <-txChan
			// Remove coinbase transaction
			if len(tx.Vin[0].Addresses) == 1 && tx.Vin[0].Addresses[0] == "coinbase" {
				continue
			}
			from, err := k.client.getFirstVinAddr(tx.Txid, tx.Vin, k.tesnet)
			if err != nil {
				continue
			}
			for _, vout := range tx.Vout {
				amount, err := common.NewAmountFromInt64(vout.Value)
				if err != nil {
					log.Info(err)
					continue
				}
				// Check script
				if len(vout.Addresses) == 0 {
					continue
				}

				newTx := common.Transaction{
					TxID:          tx.Txid,
					From:          from,
					To:            vout.Addresses[0],
					Amount:        amount,
					Currency:      common.BTC,
					Height:        0,
					Timestamp:     tx.Receivedtime,
					Confirmations: 0,
					OutputIndex:   int(vout.N),
					Spent:         false,
				}
				k.mu.Lock()
				k.txs[newTx.Serialize()] = newTx
				k.mu.Unlock()
			}
		}
	}()
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
