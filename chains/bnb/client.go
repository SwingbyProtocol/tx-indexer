package bnb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/chains/bnb/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	bnb_rpc "github.com/binance-chain/go-sdk/client/rpc"
	bnb_types "github.com/binance-chain/go-sdk/common/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

type Client struct {
	httpUrl *url.URL
	ticker  *time.Ticker
	Client  bnb_rpc.Client
}

const (
	BnbRawAddrLen = bnb_types.AddrLen

	BnbRawTxHashLen = 32

	BnbSendTxType = "cosmos-sdk/Send"

	TxTypeReceived = "RECEIVE"
	TxTypeSend     = "SEND"
)

const (
	rpcReqTimeout                     = 5 * time.Second
	transactionQueryEndpoint          = "/api/v1/txs"
	multiSendTransactionQueryEndpoint = "/api/v1/sub-tx-list"
	multiSendQueryConcurrency         = 3
)

func NewClient(rpcApi *url.URL, httpApi *url.URL, network bnb_types.ChainNetwork, optionalTickInterval ...time.Duration) *Client {
	log.Infof("BNB client connecting to (rpc: %s, http: %s)...", rpcApi, httpApi)
	client := bnb_rpc.NewRPCClient(rpcApi.Host, network)
	client.SetTimeOut(rpcReqTimeout)
	var ticker *time.Ticker
	if 0 < len(optionalTickInterval) {
		ticker = time.NewTicker(optionalTickInterval[0])
	}
	c := &Client{
		Client:  client,
		ticker:  ticker,
		httpUrl: httpApi,
	}
	return c
}

func (ps *Client) RpcClient() bnb_rpc.Client {
	return ps.Client
}

func (ps *Client) GetTransactions(params common.TxQueryParams) ([]common.Transaction, error) {
	page, pageSize := 1, 100 // 100 = max
	query := map[string]string{
		"address": params.Address,
		"txType":  "TRANSFER",
		"page":    fmt.Sprintf("%d", page),
		"rows":    fmt.Sprintf("%d", pageSize),
	}
	if params.TimeFrom != 0 {
		query["startTime"] = strconv.Itoa(int(params.TimeFrom) * 1000) // in ms
	}
	if params.TimeTo != 0 {
		query["endTime"] = strconv.Itoa(int(params.TimeTo) * 1000)
	}
	if params.Type != "" {
		query["side"] = params.Type
	}
	// fetch the raw txs from the api, iterating over the pages until we reach the end (max page size is 100)
	var allTxsRes, lastTxsRes types.TransactionsRes
	for lastTxsRes.Txs == nil || len(lastTxsRes.Txs) == pageSize { // 100 = max page size
		query["page"] = fmt.Sprintf("%d", page)
		log.Debugf("Querying BNB endpoint %s (page: %d): %+v", transactionQueryEndpoint, page, query)
		resp, err := ps.httpGet(transactionQueryEndpoint, query)
		if err != nil {
			return nil, err
		}
		// marshal from json
		// binance explorer does not use status codes (they are always 200)
		// so errors can cause for an unmarshal failure
		lastTxsRes = types.TransactionsRes{}
		if err := json.Unmarshal(resp, &lastTxsRes); err != nil {
			log.Warningf("BNB Query error - failed to marshal: %s", err)
			log.Warningf("BNB Query error - invalid json: %s", string(resp))
			return nil, err
		}
		if allTxsRes.Txs == nil {
			allTxsRes.Txs = lastTxsRes.Txs
		} else if 0 < len(lastTxsRes.Txs) {
			allTxsRes.Txs = append(allTxsRes.Txs, lastTxsRes.Txs...)
			// safety catch
			expectedAllTxLen := page * pageSize
			if len(lastTxsRes.Txs) == pageSize && len(allTxsRes.Txs) != expectedAllTxLen {
				return nil, fmt.Errorf("expected %d txs in total (page: %d) but have %d",
					expectedAllTxLen, page, len(allTxsRes.Txs))
			}
		}
		page++
	}
	expectedAllTxMaxLen := (page - 1) * pageSize // - 1 because `page` was incremented at the end of the loop
	if len(allTxsRes.Txs) > expectedAllTxMaxLen {
		return nil, fmt.Errorf("expected less than %d txs in total (page: %d) but have %d",
			expectedAllTxMaxLen, page-1, len(allTxsRes.Txs))
	}
	// get multi-send TXS
	errChan := make(chan error, len(allTxsRes.Txs))
	resChan := make(chan []types.Transaction, len(allTxsRes.Txs))
	sem := semaphore.NewWeighted(multiSendQueryConcurrency)
	transactions := make([]types.Transaction, 0, len(allTxsRes.Txs)*25) // with some buffer
	multiStarted := 0
	for i, tx := range allTxsRes.Txs {
		// if (0 < params.TimeFrom && tx.Timestamp < params.TimeFrom*1000) ||
		// 	(0 < params.TimeTo && params.TimeTo*1000 < tx.Timestamp) {
		// 	continue
		// }
		if 0 < tx.HasChildren {
			multiStarted += 1
			// request to explorer to get multi-send transactions
			go ps.getMultiSendTx(tx, sem, resChan, errChan)
			continue
		}
		tx.OutputIndex = i // the best we got
		transactions = append(transactions, tx)
	}
	if 0 < multiStarted {
		multiReceived := 0
		for {
			select {
			case err := <-errChan:
				// quit function on error and leave gb to cleanup channels
				// some routines may be writing still
				if err == nil {
					continue
				}
				return nil, err
			case res := <-resChan:
				if res == nil {
					continue
				}
				transactions = append(transactions, res...)
				multiReceived += 1
			}
			if multiReceived >= multiStarted {
				// received all request signals from routines
				break
			}
		}
	}
	close(errChan)
	close(resChan)
	// filter send/receive
	newTransactions := make([]types.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if params.Type == TxTypeSend && tx.FromAddr != params.Address {
			// only want send txs - not a send tx
			continue
		}
		if params.Type == TxTypeReceived && tx.ToAddr != params.Address {
			// only want receive txs - not a receive tx
			continue
		}
		newTransactions = append(newTransactions, tx)
	}
	transactions = newTransactions
	// flatten multi-send txs into raw array
	log.Infof("BNB Query found %d transactions.", len(transactions))
	log.Debugf("BNB Query results: %+v", transactions)
	return bnbTransactionToChainTransaction(transactions)
}

// Start the service.
// If it's already started or stopped, will return an error.
// If OnStart() returns an error, it's returned by Start()
// implements BaseService
func (ps *Client) OnStart() error {
	// ps.client is already called on create
	log.Infof("%s starting. Bnb client connected.", ps.Client)
	if ps.ticker != nil {
		go func() {
			for t := range ps.ticker.C {
				ps.Tick(t)
			}
		}()
		ps.Tick(time.Now())
	}
	return nil
}

// Stop the service.
// If it's already stopped, will return an error.
// OnStop must never error.
// implements BaseService
func (ps *Client) OnStop() {
	//log.Infof("%s stopping.", ps.String())
	err := ps.Client.Stop()
	if err != nil {
		panic(err)
	}
	if ps.ticker != nil {
		ps.ticker.Stop()
	}
}

// Reset the service.
// Panics by default - must be overwritten to enable reset.
// implements BaseService
func (ps *Client) OnReset() error {
	//log.Infof("%s resetting.", ps.String())
	return nil
}

// Tick is called each `interval` (typically 1h)
func (ps *Client) Tick(t time.Time) {
	//log.Debugf("%s tick.", ps.String())
}

// ----- //

func (ps *Client) getMultiSendTx(parent types.Transaction, sem *semaphore.Weighted, output chan []types.Transaction, errChan chan error) {
	page, pageSize := 1, 1000 // the max appears to be 1000
	if err := sem.Acquire(context.Background(), 1); err != nil {
		errChan <- err
		return
	}
	defer sem.Release(1)
	mQuery := map[string]string{
		"txHash": parent.TxHash,
		"page":   fmt.Sprintf("%d", page),
		"rows":   fmt.Sprintf("%d", pageSize),
	}
	mRes, err := ps.httpGet(multiSendTransactionQueryEndpoint, mQuery)
	if err != nil {
		log.Warningf("BNB multi-send Query error - failed to get: %s", err)
		errChan <- err
		return
	}
	var rawTxs types.MultiSendTransactionsRes
	// binance explorer does not use status codes (they are always 200)
	// so errors can cause for an unmarshal failure
	if err := json.Unmarshal(mRes, &rawTxs); err != nil {
		log.Warningf("BNB multi-send Query error - failed to unmarshal: %s", err)
		log.Debug("invalid json:", string(mRes))
		errChan <- err
		return
	}
	if pageSize <= len(rawTxs.Txs) {
		err := fmt.Errorf("bnb client saw %d multi-send outputs but our rows limit is %d", len(rawTxs.Txs), pageSize)
		errChan <- err
		return
	}
	// apply parent values to txs
	for i := range rawTxs.Txs {
		rawTxs.Txs[i].TxHash = parent.TxHash
		rawTxs.Txs[i].ConfirmBlocks = parent.ConfirmBlocks
		rawTxs.Txs[i].Memo = parent.Memo
		rawTxs.Txs[i].Timestamp = parent.Timestamp
		rawTxs.Txs[i].TxAge = parent.TxAge
		rawTxs.Txs[i].Log = parent.Log
		rawTxs.Txs[i].Code = parent.Code
		rawTxs.Txs[i].BlockHeight = parent.BlockHeight
		rawTxs.Txs[i].OutputIndex = i
	}
	output <- rawTxs.Txs
}

func bnbTransactionToChainTransaction(txs []types.Transaction) ([]common.Transaction, error) {
	newTxs := make([]common.Transaction, 0, len(txs))
	for _, tx := range txs {
		amount, err := common.NewAmountFromString(string(tx.Value))
		if err != nil {
			// ignore if we cant parse amount
			// this will be due to the currency not being supported
			continue
		}
		// ignore if haschildren=1 since that is a multisend
		// and we have already appended all multisend txs into the given
		// list
		if tx.HasChildren > 0 {
			continue
		}
		timestamp := time.Unix(0, tx.Timestamp*int64(time.Millisecond))
		assetStr := tx.TxAsset
		if assetStr == "" {
			assetStr = tx.ChildTxAsset
		}
		newTx := common.Transaction{
			TxID:          strings.ToLower(tx.TxHash),
			From:          strings.ToLower(tx.FromAddr),
			To:            strings.ToLower(tx.ToAddr),
			Amount:        amount,
			Timestamp:     timestamp,
			Currency:      common.NewSymbol(assetStr),
			Confirmations: tx.TxAge,
			Memo:          tx.Memo,
			OutputIndex:   tx.OutputIndex,
		}
		newTxs = append(newTxs, newTx)
	}
	// make sure sorted by timestamp
	sort.SliceStable(newTxs, func(i, j int) bool {
		return newTxs[i].Timestamp.Before(newTxs[j].Timestamp)
	})
	return newTxs, nil
}

func (ps *Client) httpGet(endpoint string, query map[string]string) ([]byte, error) {
	return api.Get(ps.httpUrl.String(), endpoint, query)
}
