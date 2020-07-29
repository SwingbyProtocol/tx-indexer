package btc

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	log "github.com/sirupsen/logrus"
)

const (
	TxTypeReceived = "receive"
	TxTypeSend     = "send"
	MinMempoolFees = int64(300)
)

type Client struct {
	*rpcclient.Client
	url        *url.URL
	mu         *sync.RWMutex
	sinceHash  *chainhash.Hash
	vinAddress map[string]string
}

func NewBtcClient(path string) (*Client, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	disableTLS, useLegacyHTTP := false, false
	if u.Scheme == "ws" || u.Scheme == "http" || u.Scheme == "tcp" {
		disableTLS = true
	}
	if u.Scheme == "http" || u.Scheme == "tcp" {
		useLegacyHTTP = true
	}
	pass, _ := u.User.Password()
	connCfg := &rpcclient.ConnConfig{
		Host:         u.Host,
		Endpoint:     u.Path,
		User:         u.User.Username(),
		Pass:         pass,
		HTTPPostMode: useLegacyHTTP,
		DisableTLS:   disableTLS,
	}
	nHandlers := new(rpcclient.NotificationHandlers)
	client, err := rpcclient.New(connCfg, nHandlers)
	return &Client{client, u, new(sync.RWMutex), nil, make(map[string]string)}, err
}

func (c *Client) GetBlockTxs(testNet bool, depth int) (int64, []types.Tx) {
	info, err := c.GetBlockChainInfo()
	if err != nil {
		log.Error(err)
		return 0, []types.Tx{}
	}
	if info.Blocks == 0 {
		return 0, []types.Tx{}
	}
	hash, _ := chainhash.NewHashFromStr(info.BestBlockHash)
	txs := c.GetTxs([]types.Tx{}, hash, int64(info.Blocks), depth, testNet)
	return int64(info.Blocks), txs
}

func (c *Client) TxtoCommonTx(tx types.Tx, testNet bool) ([]common.Transaction, error) {
	txs := []common.Transaction{}
	if len(tx.Vin) == 0 {
		return txs, errors.New("Tx has no input :" + tx.Txid)
	}
	// Avoid coinbase transaction
	if len(tx.Vin[0].Addresses) == 1 && tx.Vin[0].Addresses[0] == "coinbase" {
		return txs, nil
	}
	froms, fees, err := c.getVinAddrsAndFees(tx.Txid, tx.Vin, tx.Vout, testNet)
	if err != nil {
		log.Debug(err)
		return txs, nil
	}
	// Except mempool tx that hasn't minimum fees
	if fees <= MinMempoolFees && tx.Height == int64(0) {
		text := fmt.Sprintf("Skip because Tx: %s fees insufficient fees: %d expected >= %d", tx.Txid, fees, MinMempoolFees)
		return txs, errors.New(text)
	}
	time := tx.Receivedtime
	if tx.Height != int64(0) {
		time = tx.MinedTime
	}
	for _, vout := range tx.Vout {
		amount, err := common.NewAmountFromInt64(vout.Value)
		if err != nil {
			log.Warn(err)
			continue
		}
		// Check script
		if len(vout.Addresses) == 0 {
			continue
		}
		tx := common.Transaction{
			TxID:        tx.Txid,
			From:        froms[0],
			To:          vout.Addresses[0],
			Amount:      amount,
			Currency:    common.BTC,
			Height:      tx.Height,
			Timestamp:   time,
			OutputIndex: int(vout.N),
			Spent:       false,
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (c *Client) GetTxs(txs []types.Tx, hash *chainhash.Hash, height int64, depth int, testNet bool) []types.Tx {
	btcNet := &chaincfg.MainNetParams
	if testNet {
		btcNet = &chaincfg.TestNet3Params
	}
	if depth == 0 {
		return txs
	}
	depth--
	block, err := c.Client.GetBlock(hash)
	if err != nil {
		return txs
	}
	log.Infof("BTC txs scaning... block: %d", height)
	for _, tx := range block.Transactions {
		newTx := utils.MsgTxToTx(tx, btcNet)
		newTx.Height = height
		newTx.MinedTime = block.Header.Timestamp
		txs = append(txs, newTx)
	}
	height--
	return c.GetTxs(txs, &block.Header.PrevBlock, height, depth, testNet)
}

func (c *Client) getVinAddrsAndFees(txid string, vin []*types.Vin, vout []*types.Vout, testNet bool) ([]string, int64, error) {
	if len(vin) == 0 {
		return []string{}, 0, errors.New("vin is not exist")
	}
	targets := []string{}
	vinTotal := int64(0)
	for _, in := range vin {
		inTx, err := c.GetTxByTxID(in.Txid, testNet)
		if err != nil {
			text := fmt.Sprintf("%s vin: %s", err.Error(), in.Txid)
			return []string{}, 0, errors.New(text)
		}
		target := inTx.Vout[in.Vout]
		inAddress := target.Addresses[0]
		targets = append(targets, inAddress)
		vinTotal += target.Value
	}
	voutTotal := int64(0)
	for _, vout := range vout {
		voutTotal += vout.Value
	}
	fees := vinTotal - voutTotal
	return targets, fees, nil
}

func (c *Client) GetTxByTxID(txid string, testNet bool) (*types.Tx, error) {
	btcNet := &chaincfg.MainNetParams
	if testNet {
		btcNet = &chaincfg.TestNet3Params
	}
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, err
	}
	txData, err := c.GetRawTransaction(hash)
	if err != nil {
		return nil, err
	}
	tx := utils.MsgTxToTx(txData.MsgTx(), btcNet)
	return &tx, nil
}
