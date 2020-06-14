package bnb

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/binance-chain/go-sdk/common/types"
	log "github.com/sirupsen/logrus"
)

const (
	interval   = 10 * time.Second
	loadBlocks = 345600
)

type Keeper struct {
	mu          *sync.RWMutex
	client      *Client
	ticker      *time.Ticker
	watchAddr   types.AccAddress
	network     types.ChainNetwork
	testnet     bool
	accessToken string
	txs         map[string]common.Transaction
	timestamps  map[int64]time.Time
	isScanEnd   bool
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
		timestamps:  make(map[int64]time.Time),
		txs:         make(map[string]common.Transaction),
	}
	return k
}

func (k *Keeper) StartReScanAddr(addr string, timestamp int64) error {
	// TODO: set rescan
	return nil
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	watch := r.URL.Query().Get("watch")
	from := r.URL.Query().Get("height_from")
	fromNum, _ := strconv.Atoi(from)
	to := r.URL.Query().Get("height_to")
	toNum, _ := strconv.Atoi(to)
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
	txs := common.Txs{}
	for _, tx := range k.txs {
		txs = append(txs, tx)
	}
	inTxs := txs.GetRangeTxs(fromNum, toNum).Receive(watch)
	outTxs := txs.GetRangeTxs(fromNum, toNum).Send(watch)

	w.WriteJson(State{
		InTxsMempool:  []common.Transaction{},
		OutTxsMempool: []common.Transaction{},
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
	go func() {
		for {
			select {
			case <-k.ticker.C:
				if !k.client.IsActive() {
					log.Infof("bnc ws api is not active.. reset.")
					//k.client.Reset()
				}
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
	resultStatus, err := k.client.Status()
	if err != nil {
		log.Info(err)
		return
	}
	if resultStatus.SyncInfo.CatchingUp {
		// Still sync
		log.Info("bnb chain still syncing...")
	}
	if resultStatus.SyncInfo.LatestBlockHeight == 0 {
		log.Info("Sync info latest block height is zero")
		return
	}
	maxHeight, _, err := k.client.GetLatestBlockHeight()
	if err != nil {
		log.Info(err)
		return
	}
	minHeight := maxHeight - loadBlocks
	if k.isScanEnd {
		minHeight = maxHeight - 12000
	}
	if !k.isScanEnd {
		wg := new(sync.WaitGroup)
		heights := make(chan int64)
		for w := 1; w <= 200; w++ {
			go k.SyncBlockTimes(wg, heights)
		}
		for h := maxHeight; h >= minHeight; h-- {
			wg.Add(1)
			heights <- h
		}
		close(heights)
		wg.Wait()
	}
	perPage := 1000
	pageSize := 100
	k.mu.RLock()
	loadTxs := k.txs
	k.mu.RUnlock()
	for page := 1; page <= pageSize; page++ {
		txs, itemCount, _ := k.client.GetBlockTransactions(page, minHeight, maxHeight, perPage)
		for _, tx := range txs {
			loadTxs[tx.Serialize()] = tx
		}
		log.Infof("Tx scanning on the Binance chain -> total: %d, found: %d, per_page: %d, page: %d", itemCount, len(txs), perPage, page)
		//log.Info(c, page)
		pageSize = 1 + itemCount/perPage
	}
	for _, tx := range loadTxs {
		tx.Confirmations = maxHeight - tx.Height + 1
		timestamp, err := k.GetBlockTime(tx.Height)
		if err != nil {
			log.Info(err)
			continue
		}
		//log.Info(timestamp)
		tx.Timestamp = timestamp
		k.mu.Lock()
		k.txs[tx.Serialize()] = tx
		k.mu.Unlock()
	}
	k.mu.Lock()
	k.isScanEnd = true
	k.mu.Unlock()
	log.Infof("BNC txs scanning done -> loadTxs: %d", len(loadTxs))
}

func (k *Keeper) GetBlockTime(height int64) (time.Time, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.timestamps[height] != (time.Time{}) {
		return k.timestamps[height], nil
	}
	timestamp, err := k.client.GetBlockTimeStamp(height)
	if err != nil {
		return time.Time{}, err
	}
	k.timestamps[height] = timestamp
	return timestamp, nil
}

func (k *Keeper) SyncBlockTimes(wg *sync.WaitGroup, heights chan int64) {
	for height := range heights {
		timestamp, err := k.client.GetBlockTimeStamp(height)
		if err != nil {
			log.Info(err)
			continue
		}
		k.mu.Lock()
		k.timestamps[height] = timestamp
		k.mu.Unlock()
		wg.Done()
		log.Infof("Sync blocktime height: %d", height)
	}
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
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
