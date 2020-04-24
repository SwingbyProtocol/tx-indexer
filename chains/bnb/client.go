package bnb

import (
	"fmt"
	"net/url"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/binance-chain/go-sdk/client/rpc"
	"github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/binance-chain/go-sdk/types/tx"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	rpc.Client
}

func NewClient(rpcApi *url.URL, network types.ChainNetwork, period time.Duration) *Client {
	log.Infof("BNB client connecting to (rpc: %s)...", rpcApi.Host)
	client := rpc.NewRPCClient(rpcApi.Host, network)
	client.SetTimeOut(period)
	c := &Client{client}
	return c
}

func (c *Client) GetLatestBlockHeight() (int64, time.Time) {
	resultBlock, err := c.Block(nil)
	if err != nil {
		log.Info(err)
	}
	return resultBlock.Block.Height, resultBlock.Block.Time
}

func (c *Client) GetBlockTransactions(page int, minHeight int64, maxHeight int64, blockTime time.Time) ([]common.Transaction, int) {
	txs := []common.Transaction{}
	query := fmt.Sprintf("tx.height >= %d AND tx.height <= %d", minHeight, maxHeight)
	resultTxSearch, err := c.TxSearch(query, true, page, 1000)
	if err != nil {
		log.Info(err)
		return txs, 0
	}
	txs = ReultBlockToComTxs(resultTxSearch, maxHeight, blockTime)
	return txs, resultTxSearch.TotalCount
}

func ReultBlockToComTxs(resultTxSearch *rpc.ResultTxSearch, maxHeight int64, blockTime time.Time) []common.Transaction {
	newTxs := []common.Transaction{}
	for _, txData := range resultTxSearch.Txs {
		txbase := tx.StdTx{}
		base, err := rpc.ParseTx(tx.Cdc, txData.Tx)
		if err != nil {
			log.Info(err)
		}
		txbase = base.(tx.StdTx)
		thisHeight := txData.Height
		for _, message := range txbase.GetMsgs() {
			switch realMsg := message.(type) {
			// only support send msg
			case msg.SendMsg:
				if len(realMsg.Inputs) != 1 {
					continue
				}
				for i, output := range realMsg.Outputs {
					if output.Coins[0].Denom != "BTC.B-888" {
						continue
					}
					amount, err := common.NewAmountFromInt64(output.Coins[0].Amount)
					if err != nil {
						log.Info(err)
					}
					newTx := common.Transaction{
						TxID:          txData.Hash.String(),
						From:          realMsg.Inputs[0].Address.String(),
						To:            output.Address.String(),
						Amount:        amount,
						Confirmations: maxHeight - thisHeight,
						Currency:      common.BNB,
						Memo:          txbase.Memo,
						Spent:         false,
						OutputIndex:   i,
						Timestamp:     blockTime,
					}
					newTxs = append(newTxs, newTx)
				}
			}
		}
	}
	return newTxs
}

/*
	for _, block := range blocks.BlockMetas {
		if block.Header.Time.Unix() < params.TimeTo {
			heights = append(heights, block.Header.Height)
		}
		if block.Header.Time.Unix() > params.TimeFrom {
			heights = append(heights, block.Header.Height)
		}
		//log.Infof("remain block -> %d time -> %d", block.Header.Height, block.Header.Time.Unix())
	}
*/

// Stop the service.

// ----- //

/*
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
*/
