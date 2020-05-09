package btc

import (
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
	mu          *sync.RWMutex
	client      *Client
	ticker      *time.Ticker
	watchAddr   btcutil.Address
	tesnet      bool
	accessToken string
	Txs         *State
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
		Txs: &State{
			InTxs:         []common.Transaction{},
			InTxsMempool:  []common.Transaction{},
			OutTxs:        []common.Transaction{},
			OutTxsMempool: []common.Transaction{},
		},
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
		if err := res.Receive(); err != nil {
			log.Infof("bitcoind ImportMulti returned an error: %s", err)
			return
		}
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
	res := common.Response{
		Result: false,
	}
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
	w.WriteJson(k.Txs)
}

func (k *Keeper) Start() {
	if k.GetAddr() == nil {
		return
	}
	_, err := k.client.GetBlockChainInfo()
	if err != nil {
		log.Fatal(err)
	}
	// Every call try to store latest 5 blocks
	k.ticker = time.NewTicker(interval)
	k.processKeep()
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
	// Default setting
	fromTime := time.Now().Add(-48 * time.Hour)
	toTime := time.Now()
	if err := k.client.FindAndSaveSinceBlockHash(fromTime); err != nil {
		log.Warningf("bitcoind error finding best block hash: %s", err)
	}
	addr := k.GetAddr().EncodeAddress()
	// ... incoming BTC txs (mempool)
	btcInTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: addr,
		Type:    TxTypeReceived,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
		return
	}
	// ... outgoing BTC txs (mempool)
	btcOutTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: addr,
		Type:    TxTypeSend,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
		return
	}

	// ... incoming BTC txs
	btcInTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  addr,
		Type:     TxTypeReceived,
		TimeFrom: fromTime.Unix(),
		TimeTo:   toTime.Unix(), // snap to epoch interval
	}, k.tesnet)
	if err != nil {
		log.Info(err)
		return
	}

	// ... outgoing BTC txs
	btcOutTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  addr,
		Type:     TxTypeSend,
		TimeFrom: fromTime.Unix(),
		TimeTo:   toTime.Unix(),
	}, k.tesnet)
	if err != nil {
		log.Info(err)
		return
	}

	utils.SortTx(btcInTxsMempool)
	utils.SortTx(btcInTxs)
	utils.SortTx(btcOutTxsMempool)
	utils.SortTx(btcOutTxs)

	k.mu.Lock()
	k.Txs.InTxsMempool = btcInTxsMempool
	k.Txs.InTxs = btcInTxs
	k.Txs.OutTxsMempool = btcOutTxsMempool
	k.Txs.OutTxs = btcOutTxs
	k.Txs.Result = true
	k.mu.Unlock()
}

func (k *Keeper) BroadcastTx(w rest.ResponseWriter, r *rest.Request) {
	req := common.BroadcastParams{}
	res := common.BroadcastResponse{
		Result: false,
	}
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
			log.Info(tx.Txid)
		}
	}()
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
