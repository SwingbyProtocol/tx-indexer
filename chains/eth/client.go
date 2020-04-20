package eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	*ethclient.Client
	uri        string
	blockTimes map[uint64]uint64
}

type LogTransfer struct {
	From   common.Address
	To     common.Address
	Tokens *big.Int
}

type LogApproval struct {
	TokenOwner common.Address
	Spender    common.Address
	Tokens     *big.Int
}

type MempoolResponse struct {
	Result *Result `json:"result"`
}

type Result struct {
	Pending map[string]map[string]Tx `json:"pending"`
}

func NewClinet(uri string) *Client {
	client, err := ethclient.Dial(uri)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{client, uri, make(map[uint64]uint64)}
}

func (c *Client) GetMempoolTxs(tokenAddr common.Address, watchAddr common.Address) ([]types.Transaction, []types.Transaction) {
	var res MempoolResponse
	body := `{"jsonrpc":"2.0","method":"txpool_content","params":[],"id":1}`
	api := api.NewResolver(c.uri, 20)
	err := api.PostRequest("", body, &res)
	if err != nil {
		log.Info(err)
	}
	inTxs := []types.Transaction{}
	outTxs := []types.Transaction{}
	for key := range res.Result.Pending {
		base := res.Result.Pending[key]
		for key := range base {
			memTx := base[key]
			// Matched to addr
			if common.HexToAddress(memTx.To).String() != tokenAddr.String() {
				continue
			}
			tx, _, err := c.TransactionByHash(context.Background(), common.HexToHash(memTx.Hash))
			if err != nil {
				log.Info(err)
			}
			data := tx.Data()
			if len(data) != 68 {
				continue
			}
			tokenAbi, err := abi.JSON(strings.NewReader(TokenABI))
			var logTx LogTransfer
			err = tokenAbi.Unpack(&logTx, "Transfer", data[36:68])
			if err != nil {
				log.Info(err)
			}
			logTx.From = common.HexToAddress(memTx.From)
			logTx.To = common.BytesToAddress(common.TrimLeftZeroes(data[4:36]))
			amount, err := types.NewAmountFromBigIntDirect(logTx.Tokens)
			if err != nil {
				log.Info(err)
			}
			typeTx := types.Transaction{
				TxID:          tx.Hash().String(),
				From:          logTx.From.String(),
				To:            logTx.To.String(),
				Amount:        amount,
				Confirmations: 0,
				Currency:      types.ETH,
				Memo:          "",
				OutputIndex:   0,
				Spent:         false,
				Timestamp:     time.Now(),
			}
			if typeTx.From == watchAddr.String() {
				outTxs = append(outTxs, typeTx)
			}
			if typeTx.To == watchAddr.String() {
				inTxs = append(inTxs, typeTx)
			}
		}
	}
	return inTxs, outTxs
}

func (c *Client) GetTxs(tokenAddr common.Address, watchAddr common.Address) ([]types.Transaction, []types.Transaction) {
	inTxs := []types.Transaction{}
	outTxs := []types.Transaction{}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(2533531),
		ToBlock:   nil, //big.NewInt(6383840),
		Addresses: []common.Address{
			tokenAddr,
		},
	}
	logs, err := c.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	contractAbi, err := abi.JSON(strings.NewReader(TokenABI))
	if err != nil {
		log.Fatal(err)
	}
	logTransferSig := []byte("Transfer(address,address,uint256)")
	logApprovalSig := []byte("Approval(address,address,uint256)")
	logTransferSigHash := crypto.Keccak256Hash(logTransferSig)
	logApprovalSigHash := crypto.Keccak256Hash(logApprovalSig)

	latestBlock, err := c.BlockByNumber(context.Background(), nil)
	if err != nil {
		log.Info(err)
	}
	for _, vLog := range logs {
		switch vLog.Topics[0].Hex() {
		case logTransferSigHash.Hex():
			var transferEvent LogTransfer
			err := contractAbi.Unpack(&transferEvent, "Transfer", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}
			amount, err := types.NewAmountFromBigIntDirect(transferEvent.Tokens)
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
			conf := latestBlock.Number().Uint64() - vLog.BlockNumber
			from := common.HexToAddress(vLog.Topics[1].String())
			to := common.HexToAddress(vLog.Topics[2].String())

			typeTx := types.Transaction{
				TxID:          vLog.TxHash.Hex(),
				From:          from.String(),
				To:            to.String(),
				Amount:        amount,
				Confirmations: int64(conf),
				Currency:      types.ETH,
				Memo:          "",
				OutputIndex:   0,
				Spent:         false,
				Timestamp:     time.Unix(int64(c.blockTimes[vLog.BlockNumber]), 0),
			}
			if typeTx.From == watchAddr.String() {
				outTxs = append(outTxs, typeTx)
			}
			if typeTx.To == watchAddr.String() {
				inTxs = append(inTxs, typeTx)
			}

		case logApprovalSigHash.Hex():
			fmt.Printf("Log Name: Approval\n")
			var approvalEvent LogApproval
			err := contractAbi.Unpack(&approvalEvent, "Approval", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}
			approvalEvent.TokenOwner = common.HexToAddress(vLog.Topics[1].Hex())
			approvalEvent.Spender = common.HexToAddress(vLog.Topics[2].Hex())
			fmt.Printf("Token Owner: %s\n", approvalEvent.TokenOwner.Hex())
			fmt.Printf("Spender: %s\n", approvalEvent.Spender.Hex())
			fmt.Printf("Tokens: %s\n", approvalEvent.Tokens.String())
		}
	}
	log.Info("sync done")
	return inTxs, outTxs
}

func (c *Client) SetAddress(addr string) error {

	return nil
}
