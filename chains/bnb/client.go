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

func (c *Client) GetLatestBlockHeight() (int64, *time.Time, error) {
	resultBlock, err := c.Block(nil)
	if err != nil {
		return 0, nil, err
	}
	//log.Info(resultBlock)
	return resultBlock.Block.Height, &resultBlock.Block.Time, nil
}

func (c *Client) GetBlockTimeStamp(height int64) (time.Time, error) {
	blocks, err := c.BlockchainInfo(height, height)
	if err != nil {
		return time.Time{}, err
	}
	return blocks.BlockMetas[0].Header.Time, nil
}

func (c *Client) GetBlockTransactions(page int, minHeight int64, maxHeight int64, perPage int) ([]common.Transaction, int, error) {
	txs := []common.Transaction{}
	query := fmt.Sprintf("tx.height >= %d AND tx.height <= %d", minHeight, maxHeight)
	resultTxSearch, err := c.TxSearch(query, true, page, perPage)
	if err != nil {
		return txs, 0, err
	}
	txs = c.ResultBlockToComTxs(resultTxSearch, maxHeight)
	return txs, resultTxSearch.TotalCount, nil
}

func (c *Client) ResultBlockToComTxs(resultTxSearch *rpc.ResultTxSearch, maxHeight int64) []common.Transaction {
	newTxs := []common.Transaction{}
	for _, txData := range resultTxSearch.Txs {
		txbase := tx.StdTx{}
		base, err := rpc.ParseTx(tx.Cdc, txData.Tx)
		if err != nil {
			return newTxs
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
				index := 0
				for _, output := range realMsg.Outputs {
					//if output.Coins[0].Denom != "BTC.B-888" {
					//	continue
					//}
					for _, coin := range output.Coins {
						amount, err := common.NewAmountFromInt64(coin.Amount)
						if err != nil {
							log.Info(err)
							continue
						}
						currency := common.NewSymbol(coin.Denom, 8)
						newTx := common.Transaction{
							TxID:        txData.Hash.String(),
							From:        realMsg.Inputs[0].Address.String(),
							To:          output.Address.String(),
							Amount:      amount,
							Currency:    currency,
							Height:      thisHeight,
							Memo:        txbase.Memo,
							Spent:       false,
							OutputIndex: index,
							Timestamp:   time.Time{},
						}
						index++
						newTxs = append(newTxs, newTx)
					}
				}
			}
		}
	}
	return newTxs
}
