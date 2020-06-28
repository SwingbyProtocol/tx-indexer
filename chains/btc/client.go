package btc

import (
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

func (c *Client) GetBlockTxs(testNet bool, depth int) (int64, []common.Transaction) {
	info, err := c.GetBlockChainInfo()
	if err != nil {
		log.Info("error", err)
		return 0, []common.Transaction{}
	}
	if info.Blocks == 0 {
		return 0, []common.Transaction{}
	}
	hash, _ := chainhash.NewHashFromStr(info.BestBlockHash)
	rawTxs := c.GetTxs([]types.Tx{}, hash, int64(info.Blocks), depth, testNet)
	txs := []common.Transaction{}
	for _, tx := range rawTxs {
		commonTxs := c.TxtoCommonTx(tx, testNet)
		for _, comTx := range commonTxs {
			txs = append(txs, comTx)
		}
	}
	return int64(info.Blocks), txs
}

func (c *Client) TxtoCommonTx(tx types.Tx, testNet bool) []common.Transaction {
	txs := []common.Transaction{}
	if len(tx.Vin) == 0 {
		log.Errorf("Tx has no input id:%s", tx.Txid)
		return txs
	}
	// Remove coinbase transaction
	if len(tx.Vin[0].Addresses) == 1 && tx.Vin[0].Addresses[0] == "coinbase" {
		return txs
	}
	from, err := c.getFirstVinAddr(tx.Txid, tx.Vin, testNet)
	if err != nil {
		return txs
	}
	for _, vout := range tx.Vout {
		amount, err := common.NewAmountFromInt64(vout.Value)
		if err != nil {
			log.Info(err)
			continue
		}
		// Check script
		if len(vout.Addresses) == 0 {
			continue
		}
		time := tx.Receivedtime
		if tx.Height != 0 {
			time = tx.Mediantime
		}
		tx := common.Transaction{
			TxID:          tx.Txid,
			From:          from,
			To:            vout.Addresses[0],
			Amount:        amount,
			Currency:      common.BTC,
			Height:        tx.Height,
			Timestamp:     time,
			Confirmations: 0,
			OutputIndex:   int(vout.N),
			Spent:         false,
		}
		txs = append(txs, tx)
	}
	return txs
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

func (c *Client) getFirstVinAddr(txid string, vin []*types.Vin, testNet bool) (string, error) {
	c.mu.Lock()
	from := c.vinAddress[txid]
	c.mu.Unlock()
	if from != "" {
		return from, nil
	}
	inTx0, err := c.GetTxByTxID(vin[0].Txid, testNet)
	if err != nil {
		log.Warnf("%s tx: %s vin0: %s", err.Error(), txid, vin[0].Txid)
		return "", err
	}
	addr := inTx0.Vout[vin[0].Vout].Addresses[0]
	c.mu.Lock()
	c.vinAddress[txid] = addr
	c.mu.Unlock()
	return addr, nil
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
