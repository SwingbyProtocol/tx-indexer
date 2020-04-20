package btc

import (
	"sort"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/types"
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
	InTxsMempool  []types.Transaction `json:"inTxsMempool"`
	InTxs         []types.Transaction `json:"inTxs"`
	OutTxsMempool []types.Transaction `json:"outTxsMempool"`
	OutTxs        []types.Transaction `json:"outTxs"`
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
		Txs:    &State{},
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

func (k *Keeper) SetAddr(addr string) error {
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
	k.ticker = time.NewTicker(60 * time.Second)
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
	btcInTxsMempool, err := k.client.GetMempoolTransactions(types.TxQueryParams{
		Address: addr,
		Type:    TxTypeReceived,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... incoming BTC txs
	btcInTxs, err := k.client.GetTransactions(types.TxQueryParams{
		Address:  addr,
		Type:     TxTypeReceived,
		TimeFrom: fromTime.Unix(),
		TimeTo:   toTime.Unix(), // snap to epoch interval
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... outgoing BTC txs (mempool)
	btcOutTxsMempool, err := k.client.GetMempoolTransactions(types.TxQueryParams{
		Address: addr,
		Type:    TxTypeSend,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... outgoing BTC txs
	btcOutTxs, err := k.client.GetTransactions(types.TxQueryParams{
		Address:  addr,
		Type:     TxTypeSend,
		TimeFrom: fromTime.Unix(),
		TimeTo:   0, // 0 = now. always find an outbound TX even when extremely recent (anti dupe tx)
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	sortTx(btcInTxsMempool)
	sortTx(btcInTxs)
	sortTx(btcOutTxsMempool)
	sortTx(btcOutTxs)

	k.mu.Lock()
	k.Txs.InTxsMempool = btcInTxsMempool
	k.Txs.InTxs = btcInTxs
	k.Txs.OutTxsMempool = btcOutTxsMempool
	k.Txs.OutTxs = btcOutTxs
	k.mu.Unlock()
}

func (k *Keeper) BroadcastTx(w rest.ResponseWriter, r *rest.Request) {
	hex := types.BroadcastParams{}
	res := types.BroadcastResponse{
		Result: false,
	}
	err := r.DecodeJsonPayload(&hex)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	msgTx, err := DecodeToTx(hex.HEX)
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

	txChan := make(chan *Tx)
	BChan := make(chan *Block)

	nodeConfig := &NodeConfig{
		IsTestnet:        k.tesnet,
		TargetOutbound:   25,
		UserAgentName:    "Tx-indexer",
		UserAgentVersion: "1.0.0",
		TxChan:           txChan,
		BChan:            BChan,
	}
	// Node initialize
	node := NewNode(nodeConfig)
	// Node Start
	node.Start()

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

func sortTx(txs []types.Transaction) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Serialize() < txs[j].Serialize() })
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Timestamp.UnixNano() < txs[j].Timestamp.UnixNano() })
}
