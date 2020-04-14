package btc

import (
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/btcsuite/btcd/chaincfg"
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
	BtcInTxsMempool  []common.Transaction `json:"btcInTxsMempool"`
	BtcInTxs         []common.Transaction `json:"btcInTxs"`
	BtcOutTxsMempool []common.Transaction `json:"btcOutTxsMempool"`
	BtcOutTxs        []common.Transaction `json:"btcOutTxs"`
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

func (k *Keeper) SetAddr(addr string) error {
	net := &chaincfg.MainNetParams
	if k.tesnet {
		net = &chaincfg.TestNet3Params
	}
	address, err := btcutil.DecodeAddress(addr, net)
	if err != nil {
		return err
	}
	k.watchAddr = address
	return nil
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	w.WriteJson(k.Txs)
}

func (k *Keeper) Start() {
	if k.watchAddr == nil {
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
	// ... incoming BTC txs (mempool)
	btcInTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: k.watchAddr.EncodeAddress(),
		Type:    TxTypeReceived,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... incoming BTC txs
	btcInTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  k.watchAddr.EncodeAddress(),
		Type:     TxTypeReceived,
		TimeFrom: fromTime.Unix(),
		TimeTo:   toTime.Unix(), // snap to epoch interval
	})
	if err != nil {
		log.Info(err)
	}
	// ... outgoing BTC txs (mempool)
	btcOutTxsMempool, err := k.client.GetMempoolTransactions(common.TxQueryParams{
		Address: k.watchAddr.EncodeAddress(),
		Type:    TxTypeSend,
		Mempool: true,
	}, k.tesnet)
	if err != nil {
		log.Info(err)
	}
	// ... outgoing BTC txs
	btcOutTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  k.watchAddr.EncodeAddress(),
		Type:     TxTypeSend,
		TimeFrom: fromTime.Unix(),
		TimeTo:   0, // 0 = now. always find an outbound TX even when extremely recent (anti dupe tx)
	})
	if err != nil {
		log.Info(err)
	}
	k.mu.Lock()
	k.Txs.BtcInTxsMempool = btcInTxsMempool
	k.Txs.BtcInTxs = btcInTxs
	k.Txs.BtcOutTxsMempool = btcOutTxsMempool
	k.Txs.BtcOutTxs = btcOutTxs
	k.mu.Unlock()
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
