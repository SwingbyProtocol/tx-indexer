package eth

import (
	"context"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
	eth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	log "github.com/sirupsen/logrus"
)

type Keeper struct {
	mu        *sync.RWMutex
	client    *Client
	ticker    *time.Ticker
	tokenAddr eth_common.Address
	watchAddr eth_common.Address
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
	c := NewClinet(url)
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
	// TODO: set rescan
	return nil
}

func (k *Keeper) SetTokenAndAddr(token string, addr string) error {
	k.mu.Lock()
	k.tokenAddr = eth_common.HexToAddress(token)
	k.watchAddr = eth_common.HexToAddress(addr)
	k.mu.Unlock()
	return nil
}

func (k *Keeper) GetAddr() eth_common.Address {
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
	if k.GetAddr().String() == new(eth_common.Address).String() {
		log.Fatal("Error: eth address is not set")
	}
	k.ticker = time.NewTicker(15 * time.Second)
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
	var tx *types.Transaction
	rawtx, err := hexutil.Decode(req.HEX)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	rlp.DecodeBytes(rawtx, &tx)
	err = k.client.SendTransaction(context.Background(), tx)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	res.TxHash = tx.Hash().String()
	res.Result = true
	res.Msg = "success"
	w.WriteJson(res)
}

func (k *Keeper) processKeep() {
	//fromTime := time.Now().Add(-78 * time.Hour)
	//toTime := time.Now()
	inTxsMempool, outTxsMempool := k.client.GetMempoolTxs(k.tokenAddr, k.watchAddr)
	inTxs, outTxs := k.client.GetTxs(k.tokenAddr, k.watchAddr)

	utils.SortTx(inTxsMempool)
	utils.SortTx(inTxs)
	utils.SortTx(outTxsMempool)
	utils.SortTx(outTxs)

	k.mu.Lock()
	k.Txs.InTxsMempool = inTxsMempool
	k.Txs.InTxs = inTxs
	k.Txs.OutTxsMempool = outTxsMempool
	k.Txs.OutTxs = outTxs
	k.mu.Unlock()

	log.Info("ETH txs scanning done")
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
