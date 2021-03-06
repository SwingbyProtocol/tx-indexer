package bnb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	interval   = 2 * time.Second
	loadBlocks = 1100
)

type Keeper struct {
	mu         *sync.RWMutex
	client     *Client
	ticker     *time.Ticker
	network    types.ChainNetwork
	db         *common.Db
	timestamps map[int64]*time.Time
	topHeight  int64
}

func NewKeeper(urlStr string, isTestnet bool, dirPath string, pruneHours int64) *Keeper {
	bnbRPCURI, err := url.ParseRequestURI(urlStr)
	if err != nil {
		log.Fatal(err)
	}
	bnbNetwork := types.ProdNetwork
	if isTestnet {
		bnbNetwork = types.TestNetwork
	}
	db := common.NewDB()
	err = db.Start(dirPath, pruneHours)
	if err != nil {
		log.Fatal(err)
	}
	c := NewClient(bnbRPCURI, bnbNetwork, 30*time.Second)
	k := &Keeper{
		mu:         new(sync.RWMutex),
		client:     c,
		network:    bnbNetwork,
		db:         db,
		timestamps: make(map[int64]*time.Time),
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
	page := r.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(page)
	limit := r.URL.Query().Get("limit")
	limitNum, _ := strconv.Atoi(limit)
	if toNum == 0 {
		toNum = 9223372036854775807
	}
	inTxs := common.Txs{}
	inIdxs, _ := k.db.GetIdxs(watch, true)
	inIdxs = common.Idxs(inIdxs).GetRangeTxs(fromNum, toNum)
	for _, idx := range inIdxs {
		tx, err := k.db.GetTx(idx.ID)
		if err != nil {
			log.Info(err)
			continue
		}
		inTxs = append(inTxs, *tx)
	}
	outTxs := common.Txs{}
	outIdxs, _ := k.db.GetIdxs(watch, false)
	outIdxs = common.Idxs(outIdxs).GetRangeTxs(fromNum, toNum)
	for _, idx := range outIdxs {
		tx, err := k.db.GetTx(idx.ID)
		if err != nil {
			log.Info(err)
			continue
		}
		outTxs = append(outTxs, *tx)
	}
	inTxs = inTxs.Sort().Page(pageNum, limitNum)
	outTxs = outTxs.Sort().Page(pageNum, limitNum)
	k.mu.RLock()
	topHeight := k.topHeight
	k.mu.RUnlock()
	w.WriteJson(common.TxResponse{
		LatestHeight:  topHeight,
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

func (k *Keeper) GetTx(w rest.ResponseWriter, r *rest.Request) {
	txHash := r.URL.Query().Get("tx_hash")
	txs := []common.Transaction{}
	for i := 0; i <= 10000; i++ {
		key := fmt.Sprintf("%s;%d;", strings.ToUpper(txHash), i)
		tx, err := k.db.GetTx(key)
		if err != nil {
			w.WriteJson(txs)
			return
		}
		txs = append(txs, *tx)
	}
	w.WriteJson(txs)
}

func (k *Keeper) GetMemoTxs(w rest.ResponseWriter, r *rest.Request) {
	txs := []common.Transaction{}
	memo := r.URL.Query().Get("memo")
	page := r.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(page)
	limit := r.URL.Query().Get("limit")
	limitNum, _ := strconv.Atoi(limit)
	txKeys, _ := k.db.GetMemoTxs(memo)
	for _, key := range txKeys {
		tx, err := k.db.GetTx(key)
		if err != nil {
			continue
		}
		txs = append(txs, *tx)
	}
	txs = common.Txs(txs).Sort().Page(pageNum, limitNum)
	w.WriteJson(txs)
}

func (k *Keeper) Start() {
	k.ticker = time.NewTicker(interval)
	k.processKeep()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				if !k.client.IsActive() {
					log.Debugf("bnc ws api is not active.. reset.")
					//k.client.Reset()
				}
				k.processKeep()
			}
		}
	}()
}

func (k *Keeper) processKeep() {
	maxHeight, _, err := k.client.GetLatestBlockHeight()
	if err != nil {
		log.Error(err)
		return
	}
	if maxHeight == 0 {
		log.Warnf("Sync info latest block height is zero")
		return
	}
	k.mu.Lock()
	k.topHeight = maxHeight
	k.mu.Unlock()
	minHeight := maxHeight - loadBlocks
	perPage := 1000
	pageSize := 100
	for page := 1; page <= pageSize; page++ {
		txs, itemCount, _ := k.client.GetBlockTransactions(page, minHeight, maxHeight, perPage)
		k.StoreTxs(txs)
		log.Infof("Tx scanning on the BNC => maxHeight: %d, minHeight: %d, total: %d, per_page: %d, page: %d, found: %d", maxHeight, minHeight, itemCount, perPage, page, len(txs))
		pageSize = 1 + itemCount/perPage
	}
}

func (k *Keeper) StoreTxs(txs []common.Transaction) {
	for _, tx := range txs {
		_, err := k.db.GetTx(tx.Serialize())
		if err == nil {
			// tx is already exist
			continue
		}
		timestamp, err := k.GetBlockTimeStampWithCache(tx.Height)
		if err != nil {
			log.Error(err)
			continue
		}
		tx.Timestamp = timestamp
		err = k.db.StoreTx(tx.Serialize(), tx)
		if err != nil {
			log.Error(err)
			continue
		}
		k.db.StoreIdx(tx.Serialize(), &tx, true)
		k.db.StoreIdx(tx.Serialize(), &tx, false)
		// Add self send
		// if tx.From == tx.To {
		// 	k.db.StoreSelfTxkeys(tx.Serialize())
		// }
		if tx.Memo != "" {
			k.db.StoreMemoTxs(tx.Memo, tx.Serialize())
			log.Infof("Tx %s has a memo : %s", tx.Serialize(), tx.Memo)
		}
	}
}

func (k *Keeper) GetBlockTimeStampWithCache(height int64) (time.Time, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.timestamps[height] != nil {
		return *k.timestamps[height], nil
	}
	timestamp, err := k.client.GetBlockTimeStamp(height)
	if err != nil {
		return time.Time{}, err
	}
	k.timestamps[height] = &timestamp
	return timestamp, nil
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
