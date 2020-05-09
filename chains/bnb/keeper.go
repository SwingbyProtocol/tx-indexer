package bnb

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/binance-chain/go-sdk/common/types"
	log "github.com/sirupsen/logrus"
)

const (
	interval = 10 * time.Second
)

type Keeper struct {
	mu          *sync.RWMutex
	client      *Client
	ticker      *time.Ticker
	watchAddr   types.AccAddress
	network     types.ChainNetwork
	testnet     bool
	accessToken string
	Txs         *State
	isScanEnd   bool
	cache       map[string]common.Transaction
}

type State struct {
	common.Response
	InTxsMempool  []common.Transaction `json:"inTxsMempool"`
	InTxs         []common.Transaction `json:"inTxs"`
	OutTxsMempool []common.Transaction `json:"outTxsMempool"`
	OutTxs        []common.Transaction `json:"outTxs"`
}

func NewKeeper(urlStr string, isTestnet bool, accessToken string) *Keeper {
	bnbRPCURI, err := url.ParseRequestURI(urlStr)
	if err != nil {
		log.Fatal(err)
	}
	bnbNetwork := types.ProdNetwork
	if isTestnet {
		bnbNetwork = types.TestNetwork
	}
	c := NewClient(bnbRPCURI, bnbNetwork, 30*time.Second)
	k := &Keeper{
		mu:          new(sync.RWMutex),
		client:      c,
		testnet:     isTestnet,
		network:     bnbNetwork,
		accessToken: accessToken,
		cache:       make(map[string]common.Transaction),
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

func (k *Keeper) SetWatchAddr(addr string, rescan bool) error {
	k.mu.Lock()
	types.Network = k.network
	accAddr, err := types.AccAddressFromBech32(addr)
	if err != nil {
		log.Info(err)
		k.mu.Unlock()
		return err
	}
	k.watchAddr = accAddr
	if rescan {
		k.isScanEnd = false
	}
	log.Infof("set bnb watch address -> %s isTestnet: %t rescan: %t", k.watchAddr.String(), k.testnet, rescan)
	k.mu.Unlock()
	return nil
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
	err = k.SetWatchAddr(req.Address, true)
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
	if !k.isScanEnd {
		res := common.Response{
			Result: false,
			Msg:    "re-scanning",
		}
		w.WriteJson(res)
		return
	}
	w.WriteJson(k.Txs)
}

func (k *Keeper) Start() {
	k.ticker = time.NewTicker(interval)
	k.processKeep()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				if !k.client.IsActive() {
					log.Infof("bnc ws api is not active.. reset.")
					//k.client.Reset()
				}
				k.processKeep()
			}
		}
	}()
}

func (k *Keeper) processKeep() {
	txList := []common.Transaction{}
	maxHeight, blockTime, err := k.client.GetLatestBlockHeight()
	if err != nil {
		log.Info(err)
		return
	}
	// set 48 hours
	minHeight := maxHeight - 345600
	if k.isScanEnd {
		minHeight = maxHeight - 12000
	}
	perPage := 500
	pageSize := 100
	for page := 1; page <= pageSize; page++ {
		txs, itemCount, _ := k.client.GetBlockTransactions(page, minHeight, maxHeight, perPage, *blockTime)
		for _, tx := range txs {
			txList = append(txList, tx)
		}
		log.Infof("Tx scanning on the Binance chain -> total: %d, found: %d, per_page: %d, page: %d", itemCount, len(txs), perPage, page)
		//log.Info(c, page)
		pageSize = 1 + itemCount/perPage
	}
	for _, tx := range txList {
		k.mu.Lock()
		k.cache[tx.Serialize()] = tx
		k.mu.Unlock()
	}
	k.mu.RLock()
	loadTxs := k.cache
	k.mu.RUnlock()
	inTxs := []common.Transaction{}
	outTxs := []common.Transaction{}
	for _, tx := range loadTxs {
		if tx.From == k.watchAddr.String() {
			tx.Confirmations = maxHeight - tx.Height
			outTxs = append(outTxs, tx)
		}
		if tx.To == k.watchAddr.String() {
			tx.Confirmations = maxHeight - tx.Height
			inTxs = append(inTxs, tx)
		}
	}
	utils.SortTx(inTxs)
	utils.SortTx(outTxs)
	k.mu.Lock()
	k.Txs.InTxs = inTxs
	k.Txs.OutTxs = outTxs
	k.isScanEnd = true
	k.Txs.Result = true
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
