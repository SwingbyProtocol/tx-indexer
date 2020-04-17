package eth

import (
	"context"
	"math/big"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	*ethclient.Client
	uri string
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
	return &Client{client, uri}
}

func (c *Client) GetMempoolTxs(toAddress string) {
	var res MempoolResponse
	body := `{"jsonrpc":"2.0","method":"txpool_content","params":[],"id":1}`
	api := api.NewResolver(c.uri, 20)
	err := api.PostRequest("", body, &res)
	if err != nil {
		log.Info(err)
	}
	//txs := []Tx{}
	for key := range res.Result.Pending {
		base := res.Result.Pending[key]
		for key := range base {
			tx := base[key]
			// Matched to addr
			if tx.To != toAddress {
				continue
			}
			log.Info(tx.To)
			hash := common.HexToHash(tx.Hash)
			getTx, _, err := c.TransactionByHash(context.Background(), hash)
			if err != nil {
				log.Info(err)
			}
			input := getTx.Data()
			log.Info(len(input))
			if len(input) != 68 {
				continue
			}
			prefix := common.Hex2Bytes("a9059cbb")
			transferPrefix := input[:4]
			if string(prefix) != string(transferPrefix) {
				continue
			}
			toAddr := input[5:36]
			amount := input[36:68]

			amt, err := hexutil.DecodeUint64("0x" + common.Bytes2Hex(common.TrimLeftZeroes(amount)))
			if err != nil {
				log.Info(err)
			}
			log.Info(common.BytesToAddress(toAddr).String(), " ", amt)

			txJ, _ := getTx.MarshalJSON()
			log.Info(tx.From, string(txJ))
		}
	}
}

func (c *Client) SetAddress(addr string) error {

	return nil
}
