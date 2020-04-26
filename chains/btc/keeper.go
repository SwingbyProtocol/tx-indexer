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

type Keeper struct {
	mu        *sync.RWMutex
	client    *Client
	ticker    *time.Ticker
	watchAddr btcutil.Address
	tesnet    bool
	Txs       *State
}

type State struct {
	InTxsMempool  []common.Transaction `json:"inTxsMempool"`
	InTxs         []common.Transaction `json:"inTxs"`
	OutTxsMempool []common.Transaction `json:"outTxsMempool"`
	OutTxs        []common.Transaction `json:"outTxs"`
}

func NewKeeper(url string, isTestnet bool) *Keeper {
	c, err := NewBtcClient(url)
	if err != nil {
		log.Fatal(err)
	}
	k := &Keeper{
		mu:     new(sync.RWMutex),
		client: c,
		tesnet: isTestnet,
		Txs: &State{
			InTxs:         []common.Transaction{},
			InTxsMempool:  []common.Transaction{},
			OutTxs:        []common.Transaction{},
			OutTxsMempool: []common.Transaction{},
		},
	}
	return k
}

func (k *Keeper) StartReScanAddr(addr string, timestamp int64) error {
	// set rescan
	opts := []btcjson.ImportMultiRequest{{
		ScriptPubKey: btcjson.ImportMultiRequestScriptPubKey{Address: k.watchAddr.EncodeAddress()},
		Timestamp:    &timestamp,
		WatchOnly:    true,
	}}
	res := k.client.ImportMultiRescanAsync(opts, true)
	// wait for the async importaddress result; log out any error that comes through
	go func(res rpcclient.FutureImportAddressResult) {
		log.Debugf("bitcoind ImportMulti rescan begin: %s", k.watchAddr.EncodeAddress())
		if err := res.Receive(); err != nil {
			log.Debugf("bitcoind ImportMulti returned an error: %s", err)
			return
		}
		log.Debugf("bitcoind ImportMulti rescan done: %s", k.watchAddr.EncodeAddress())
	}(res)
	return nil
}

func (k *Keeper) SetWatchAddr(addr string) error {
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
	log.Infof("set btc watch address -> %s", k.watchAddr.String())
	k.mu.Unlock()
	return nil
}

func (k *Keeper) GetAddr() btcutil.Address {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.watchAddr
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
	k.ticker = time.NewTicker(80 * time.Second)
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
	fromTime := time.Now().Add(-48 * time.Hour)
	toTime := time.Now()
	if err := k.client.FindAndSaveSinceBlockHash(fromTime); err != nil {
		log.Warningf("bitcoind error finding best block hash: %s", err)
	}
	k.mu.RLock()
	addr := k.GetAddr().EncodeAddress()
	k.mu.RUnlock()
	// ... incoming BTC txs (mempool)
	btcInTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: addr,
		Type:    TxTypeReceived,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
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
	}
	// ... outgoing BTC txs (mempool)
	btcOutTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: addr,
		Type:    TxTypeSend,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... outgoing BTC txs
	btcOutTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  addr,
		Type:     TxTypeSend,
		TimeFrom: fromTime.Unix(),
		TimeTo:   0, // 0 = now. always find an outbound TX even when extremely recent (anti dupe tx)
	}, k.tesnet)
	if err != nil {
		log.Info(err)
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
	k.mu.Unlock()
}

func (k *Keeper) BroadcastTx(w rest.ResponseWriter, r *rest.Request) {
	hex := common.BroadcastParams{}
	res := common.BroadcastResponse{
		Result: false,
	}
	err := r.DecodeJsonPayload(&hex)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	msgTx, err := utils.DecodeToTx(hex.HEX)
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
