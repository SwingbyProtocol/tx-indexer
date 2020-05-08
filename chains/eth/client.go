package eth

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/chains/eth/token"
	"github.com/SwingbyProtocol/tx-indexer/chains/eth/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	eth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	*ethclient.Client
	uri         string
	blockTimes  map[uint64]uint64
	latestBlock int64
}

func NewClinet(uri string) *Client {
	client, err := ethclient.Dial(uri)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{client, uri, make(map[uint64]uint64), 0}
}

func (c *Client) GetMempoolTxs(tokenAddr eth_common.Address, watchAddr eth_common.Address) ([]common.Transaction, []common.Transaction) {
	inTxs := []common.Transaction{}
	outTxs := []common.Transaction{}
	var res types.MempoolResponse
	body := `{"jsonrpc":"2.0","method":"txpool_content","params":[],"id":1}`
	api := api.NewResolver(c.uri, 20)
	err := api.PostRequest("", body, &res)
	if err != nil {
		log.Info(err)
		return inTxs, outTxs
	}
	for key := range res.Result.Pending {
		base := res.Result.Pending[key]
		for key := range base {
			memTx := base[key]
			// Matched to addr
			if eth_common.HexToAddress(memTx.To).String() != tokenAddr.String() {
				continue
			}
			rawTx, _, err := c.TransactionByHash(context.Background(), eth_common.HexToHash(memTx.Hash))
			if err != nil {
				log.Info(err)
			}
			data := rawTx.Data()
			if len(data) != 68 {
				continue
			}
			tokenAbi, err := abi.JSON(strings.NewReader(token.TokenABI))
			if err != nil {
				log.Info(err)
				continue
			}
			var logTx types.LogTransfer
			err = tokenAbi.Unpack(&logTx, "Transfer", data[36:68])
			if err != nil {
				log.Info(err)
			}
			logTx.From = eth_common.HexToAddress(memTx.From)
			logTx.To = eth_common.BytesToAddress(eth_common.TrimLeftZeroes(data[4:36]))
			amount, err := common.NewAmountFromBigIntDirect(logTx.Tokens)
			if err != nil {
				log.Info(err)
			}
			currency := common.NewSymbol("Sample Token", 18)
			tx := common.Transaction{
				TxID:          rawTx.Hash().String(),
				From:          logTx.From.String(),
				To:            logTx.To.String(),
				Amount:        amount,
				Currency:      currency,
				Confirmations: 0,
				Memo:          "",
				OutputIndex:   0,
				Spent:         false,
				Timestamp:     time.Unix(0, 0),
			}
			if tx.From == watchAddr.String() {
				outTxs = append(outTxs, tx)
			}
			if tx.To == watchAddr.String() {
				inTxs = append(inTxs, tx)
			}
		}
	}
	return inTxs, outTxs
}

func (c *Client) SyncLatestBlocks() error {
	latestBlock, err := c.BlockByNumber(context.Background(), nil)
	if err != nil {
		return err
	}
	c.latestBlock = latestBlock.Header().Number.Int64()
	return nil
}

func (c *Client) GetTxs(tokenAddr eth_common.Address, watchAddr eth_common.Address) ([]common.Transaction, []common.Transaction) {
	inTxs := []common.Transaction{}
	outTxs := []common.Transaction{}
	// default 48 hours
	fromHeight := int64(172800 / 12)
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(c.latestBlock - fromHeight),
		ToBlock:   nil, //big.NewInt(6383840),
		Addresses: []eth_common.Address{
			tokenAddr,
		},
	}
	logs, err := c.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	contractAbi, err := abi.JSON(strings.NewReader(token.TokenABI))
	if err != nil {
		log.Fatal(err)
	}
	logTransferSig := []byte("Transfer(address,address,uint256)")
	logApprovalSig := []byte("Approval(address,address,uint256)")
	logTransferSigHash := crypto.Keccak256Hash(logTransferSig)
	logApprovalSigHash := crypto.Keccak256Hash(logApprovalSig)
	for _, vLog := range logs {
		switch vLog.Topics[0].Hex() {
		case logTransferSigHash.Hex():
			var transferEvent types.LogTransfer
			err := contractAbi.Unpack(&transferEvent, "Transfer", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}
			amount, err := common.NewAmountFromBigIntDirect(transferEvent.Tokens)
			if err != nil {
				log.Info(err)
			}
			if c.blockTimes[vLog.BlockNumber] == 0 {
				block, err := c.BlockByNumber(context.Background(), new(big.Int).SetUint64(vLog.BlockNumber))
				if err != nil {
					log.Info(err)
				}
				c.blockTimes[vLog.BlockNumber] = block.Time()
			}
			conf := uint64(c.latestBlock) - vLog.BlockNumber
			from := eth_common.HexToAddress(vLog.Topics[1].String())
			to := eth_common.HexToAddress(vLog.Topics[2].String())
			currency := common.NewSymbol("Sample Token", 18)
			tx := common.Transaction{
				TxID:          vLog.TxHash.Hex(),
				From:          from.String(),
				To:            to.String(),
				Amount:        amount,
				Currency:      currency,
				Confirmations: int64(conf),
				Memo:          "",
				OutputIndex:   0,
				Spent:         false,
				Timestamp:     time.Unix(int64(c.blockTimes[vLog.BlockNumber]), 0),
			}
			if tx.From == watchAddr.String() {
				outTxs = append(outTxs, tx)
			}
			if tx.To == watchAddr.String() {
				inTxs = append(inTxs, tx)
			}

		case logApprovalSigHash.Hex():
			// Approval is not support
		}
	}
	return inTxs, outTxs
}
