package eth

import (
	"sort"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

type Keeper struct {
	mu        *sync.RWMutex
	client    *Client
	ticker    *time.Ticker
	tokenAddr common.Address
	watchAddr common.Address
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
	c := NewClinet(url)
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
	return nil
}

func (k *Keeper) SetTokenAndAddr(token string, addr string) error {
	k.mu.Lock()
	k.tokenAddr = common.HexToAddress(token)
	k.watchAddr = common.HexToAddress(addr)
	k.mu.Unlock()
	return nil
}

func (k *Keeper) GetAddr() common.Address {
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
	if k.GetAddr().String() == new(common.Address).String() {
		log.Fatal("Error: eth address is not set")
	}
	// Every call try to store latest 5 blocks
	k.ticker = time.NewTicker(2 * time.Second)
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
	//fromTime := time.Now().Add(-78 * time.Hour)
	//toTime := time.Now()

	inTxsMempool, outTxsMempool := k.client.GetMempoolTxs(k.tokenAddr, k.watchAddr)

	inTxs, outTxs := k.client.GetTxs(k.tokenAddr, k.watchAddr)

	sortTx(inTxsMempool)
	sortTx(inTxs)
	sortTx(outTxsMempool)
	sortTx(outTxs)

	k.mu.Lock()
	k.Txs.InTxsMempool = inTxsMempool
	k.Txs.InTxs = inTxs
	k.Txs.OutTxsMempool = outTxsMempool
	k.Txs.OutTxs = outTxs
	k.mu.Unlock()
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}

func sortTx(txs []types.Transaction) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Serialize() < txs[j].Serialize() })
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Timestamp.UnixNano() < txs[j].Timestamp.UnixNano() })
}
