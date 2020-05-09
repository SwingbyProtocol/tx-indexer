package eth

import (
	"context"
	"errors"
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

const (
	interval = 15 * time.Second
)

type Keeper struct {
	mu          *sync.RWMutex
	client      *Client
	ticker      *time.Ticker
	accessToken string
	tokenAddr   eth_common.Address
	watchAddr   eth_common.Address
	tesnet      bool
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
	c := NewClinet(url)
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
	err = k.SetTokenAndWatchAddr(req.TargetToken, req.Address)
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

func (k *Keeper) SetTokenAndWatchAddr(token string, addr string) error {
	checkAddr := eth_common.IsHexAddress(addr)
	checkToken := eth_common.IsHexAddress(token)
	if !checkAddr || !checkToken {
		return errors.New("watch address or target token address is not valid")
	}
	k.mu.Lock()
	k.tokenAddr = eth_common.HexToAddress(token)
	k.watchAddr = eth_common.HexToAddress(addr)
	log.Infof("set eth watch address -> %s", k.watchAddr.String())
	log.Infof("set erc20 token address -> %s", k.tokenAddr.String())
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
	err := k.client.SyncLatestBlocks()
	if err != nil {
		log.Info(err)
		return
	}
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
	k.Txs.Result = true
	k.mu.Unlock()

	log.Info("ETH txs scanning done")
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
