package bnb

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/binance-chain/go-sdk/common/types"
	log "github.com/sirupsen/logrus"
)

type Keeper struct {
	mu        *sync.RWMutex
	client    *Client
	ticker    *time.Ticker
	watchAddr types.AccAddress
	network   types.ChainNetwork
	testnet   bool
	Txs       *State
}

type State struct {
	InTxsMempool  []common.Transaction `json:"inTxsMempool"`
	InTxs         []common.Transaction `json:"inTxs"`
	OutTxsMempool []common.Transaction `json:"outTxsMempool"`
	OutTxs        []common.Transaction `json:"outTxs"`
}

func NewKeeper(urlStr string, isTestnet bool) *Keeper {
	bnbRPCURI, err := url.ParseRequestURI(urlStr)
	if err != nil {
		log.Fatal(err)
	}
	bnbNetwork := types.ProdNetwork
	if isTestnet {
		bnbNetwork = types.TestNetwork
	}
	c := NewClient(bnbRPCURI, bnbNetwork, 2*time.Second)
	k := &Keeper{
		mu:      new(sync.RWMutex),
		client:  c,
		testnet: isTestnet,
		network: bnbNetwork,
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

func (k *Keeper) SetWatchAddr(addr string) error {
	k.mu.Lock()
	types.Network = k.network
	accAddr, err := types.AccAddressFromBech32(addr)
	if err != nil {
		log.Info(err)
	}
	k.watchAddr = accAddr
	log.Infof("set bnb watch address -> %s", k.watchAddr.String())
	k.mu.Unlock()
	return nil
}

/*
func (k *Keeper) GetAddr() types.AccAddress {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.watchAddr
}

*/
func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	w.WriteJson(k.Txs)
}

func (k *Keeper) Start() {
	k.ticker = time.NewTicker(10 * time.Second)
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
	txList := []common.Transaction{}
	maxHeight, blockTime := k.client.GetLatestBlockHeight()
	minHeight := maxHeight - 832000
	txs, itemCount := k.client.GetBlockTransactions(1, minHeight, maxHeight, blockTime)
	for _, tx := range txs {
		txList = append(txList, tx)
	}
	//log.Info(itemCount)
	pageSize := 1 + itemCount/1000
	for page := 2; page <= pageSize; page++ {
		txs, _ := k.client.GetBlockTransactions(page, minHeight, maxHeight, blockTime)
		for _, tx := range txs {
			txList = append(txList, tx)
		}
		//log.Info(c, page)
	}
	inTxs := []common.Transaction{}
	outTxs := []common.Transaction{}
	for _, tx := range txList {
		if tx.From == k.watchAddr.String() {
			outTxs = append(outTxs, tx)
		}
		if tx.To == k.watchAddr.String() {
			inTxs = append(inTxs, tx)
		}
	}
	k.mu.Lock()
	k.Txs.InTxs = inTxs
	k.Txs.OutTxs = outTxs
	k.mu.Unlock()

	log.Info("BNC txs scanning done")
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
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
	signedTx, err := hex.DecodeString(req.HEX)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	result, err := k.client.BroadcastTxSync(signedTx)
	if err != nil {
		res.Msg = err.Error()
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	log.Info(result)
	digest := sha256.Sum256(signedTx)
	res.TxHash = strings.ToLower(hex.EncodeToString(digest[:]))
	res.Result = true
	res.Msg = "success"
	w.WriteJson(res)
}
