package eth

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
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
	mu            *sync.RWMutex
	client        *Client
	ticker        *time.Ticker
	tokenAddr     eth_common.Address
	tokenName     string
	tokenDecimals int
	tesnet        bool
	isScanEnd     bool
	txs           map[string]common.Transaction
}

func NewKeeper(url string, isTestnet bool) *Keeper {
	c := NewClinet(url)
	k := &Keeper{
		mu:        new(sync.RWMutex),
		client:    c,
		tesnet:    isTestnet,
		isScanEnd: false,
		txs:       make(map[string]common.Transaction),
	}
	return k
}

func (k *Keeper) SetToken(token string, tokenName string, decimlas int) {
	k.tokenAddr = eth_common.HexToAddress(token)
	k.tokenName = tokenName
	k.tokenDecimals = decimlas
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	k.mu.RLock()
	defer k.mu.RUnlock()
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
		toNum = 100000000
	}
	txs := common.Txs{}
	k.mu.RLock()
	isScan := k.isScanEnd
	for _, tx := range k.txs {
		txs = append(txs, tx)
	}
	k.mu.RUnlock()
	if !isScan {
		res := common.Response{
			Result: false,
			Msg:    "re-scanning",
		}
		w.WriteHeader(400)
		w.WriteJson(res)
		return
	}
	rangeTxs := txs.GetRangeTxs(fromNum, toNum).Sort()
	memPoolTxs := txs.GetRangeTxs(0, 0).Sort()

	watchAddr := eth_common.HexToAddress(watch).String()

	inTxsMemPool := memPoolTxs.ReceiveMempool(watchAddr).Page(pageNum, limitNum)
	outTxsMemPool := memPoolTxs.SendMempool(watchAddr).Page(pageNum, limitNum)
	inTxs := rangeTxs.Receive(watchAddr).Page(pageNum, limitNum)
	outTxs := rangeTxs.Send(watchAddr).Page(pageNum, limitNum)

	w.WriteJson(common.TxResponse{
		InTxsMempool:  inTxsMemPool,
		OutTxsMempool: outTxsMemPool,
		InTxs:         inTxs,
		OutTxs:        outTxs,
		Response: common.Response{
			Msg:    "",
			Result: true,
		},
	})
}

func (k *Keeper) UpdateTxs() {
	deleteList := []string{}
	k.mu.RLock()
	txs := k.txs
	k.mu.RUnlock()
	for _, tx := range txs {
		if tx.Timestamp.Add(48*time.Hour).Unix() < time.Now().Unix() {
			deleteList = append(deleteList, tx.TxID)
			continue
		}
	}
	for _, txID := range deleteList {
		k.mu.Lock()
		delete(k.txs, txID)
		k.mu.Unlock()
	}
}

func (k *Keeper) Start() {
	k.ticker = time.NewTicker(interval)
	k.processKeep()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				k.processKeep()
				k.UpdateTxs()
			}
		}
	}()
}

func (k *Keeper) processKeep() {
	err := k.client.SyncLatestBlocks()
	if err != nil {
		log.Info(err)
		return
	}
	// default 48 hours
	blocks := int64(172800 / 10)
	k.mu.RLock()
	if k.isScanEnd {
		blocks = int64(7200 / 10)
	}
	k.mu.RUnlock()

	txsMempool := k.client.GetMempoolTxs(k.tokenAddr, k.tokenName, k.tokenDecimals)
	txs := k.client.GetTxs(k.tokenAddr, blocks, k.tokenName, k.tokenDecimals)
	for _, tx := range txsMempool {
		txs = append(txs, tx)
	}
	k.mu.Lock()
	for _, tx := range txs {
		k.txs[tx.Serialize()] = tx
	}
	for _, tx := range k.txs {
		tx.Confirmations = k.client.latestBlock - tx.Height
	}
	k.isScanEnd = true
	log.Infof("ETH txs scanning done token is %s loadTxs: %d mempool: %d", k.tokenAddr.String(), len(k.txs), len(txsMempool))
	k.mu.Unlock()
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

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
