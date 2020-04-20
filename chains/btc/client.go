package btc

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const (
	TxTypeReceived = "receive"
	TxTypeSend     = "send"
)

type Client struct {
	*rpcclient.Client
	url       *url.URL
	sinceHash *chainhash.Hash
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
	return &Client{client, u, nil}, err
}

// FindAndSaveSinceBlockHash tries to find the hash of the first block where the timestamp is greater or equal to `fromTime`,
// which represents the start time of the window of transactions that we are willing to look at on the blockchain.
// This is a performance optimization to try to make lighter listsinceblock queries to the bitcoind node.
func (c *Client) FindAndSaveSinceBlockHash(fromTime time.Time) error {
	log.Infof("bitcoind performing binary search to find best block height for %s", fromTime)
	info, err := c.GetBlockChainInfo()
	if err != nil {
		return err
	}
	pruneHeight := 0
	if 0 < info.PruneHeight {
		pruneHeight = int(info.PruneHeight)
	}
	log.Infof("bitcoind binary search to find the best block; pruneHeight is: %d", pruneHeight)
	errored := false
	bestSinceHeight := int32(sort.Search(int(info.Blocks)-pruneHeight, func(height int) bool {
		queryHeight := int64(height + pruneHeight)
		log.Infof("bitcoind getting block hash for height %d", queryHeight)
		hash, err := c.GetBlockHash(queryHeight)
		if err != nil {
			log.Warningf("bitcoind error while getting block hash for height %d: %s", queryHeight, err)
			errored = true
			return false
		}
		log.Debugf("bitcoind getting block header for height %d", queryHeight)
		header, err := c.GetBlockHeader(hash)
		if err != nil {
			log.Warningf("bitcoind error while getting block header for height %d (hash=%s): %s", queryHeight, hash, err)
			errored = true
			return false
		}
		log.Debugf("bitcoind block timestamp for height %d: %s", queryHeight, header.Timestamp)
		return header.Timestamp.Unix() >= fromTime.Unix()
	}))
	if errored {
		return errors.New("bitcoind finding the best block hash for listsinceblock failed")
	}
	bestSinceHeight = bestSinceHeight + int32(pruneHeight)
	bestSinceHash, err := c.GetBlockHash(int64(bestSinceHeight) - 3)
	if err != nil {
		return err
	}
	log.Infof("bitcoind best height to query transactions from: %d, hash: %s", bestSinceHeight, bestSinceHash)
	c.sinceHash = bestSinceHash
	return nil
}

func (c *Client) GetTransactions(params common.TxQueryParams, testNet bool) ([]common.Transaction, error) {
	var err error
	allTxs := make(map[string]*btcjson.GetTransactionResult, 1000)
	batchTxs, err := c.ListSinceBlockMinConf(c.sinceHash, 1, true)
	if err != nil {
		return nil, err
	}
	for _, tx := range batchTxs.Transactions {
		if (params.Mempool && tx.Confirmations > 0) ||
			(0 < params.TimeFrom && tx.BlockTime < params.TimeFrom) ||
			(0 < params.TimeTo && params.TimeTo < tx.BlockTime) {
			continue
		}
		if _, ok := allTxs[tx.TxID]; ok { // already seen?
			continue
		}
		hash, err := chainhash.NewHashFromStr(tx.TxID)
		if err != nil {
			// should never happen 💦
			continue
		}
		// TODO: do this async
		if allTxs[tx.TxID], err = c.GetTransaction(hash, true); err != nil {
			return nil, err
		}
	}
	log.Infof("bitcoind returned %d txs for GetTransactions", len(allTxs))
	return c.btcTxsToCmnTransactions(params, allTxs, testNet)
}

func (c *Client) GetMempoolTransactions(params common.TxQueryParams, testNet bool) ([]common.Transaction, error) {
	unspents, err := c.GetSpendableOutputs(params, testNet, 0, 0)
	if err != nil {
		return nil, err
	}
	btcNet := &chaincfg.MainNetParams
	if testNet {
		btcNet = &chaincfg.TestNet3Params
	}
	txs := make([]Tx, len(unspents))
	for _, unspent := range unspents {
		hash, _ := chainhash.NewHashFromStr(unspent.TxID)
		txData, _ := c.GetRawTransaction(hash)
		tx := MsgTxToTx(txData.MsgTx(), btcNet)
		txs = append(txs, tx)
	}
	parsedTxs, err := btcTransactionsToChainTransactions(0, txs, params.TimeFrom, params.TimeTo)
	if err != nil {
		return nil, err
	}
	finalTxs := make([]common.Transaction, 0, len(parsedTxs))
	for _, parsedTx := range parsedTxs {
		if (params.Type == TxTypeSend && parsedTx.From != params.Address) ||
			(params.Type == TxTypeReceived && parsedTx.To != params.Address) {
			continue
		}
		finalTxs = append(finalTxs, parsedTx)
	}
	return finalTxs, nil
}

func (c *Client) GetTxByTxID(txid string, testNet bool) (*Tx, error) {
	btcNet := &chaincfg.MainNetParams
	if testNet {
		btcNet = &chaincfg.TestNet3Params
	}
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, err
	}
	txData, err := c.GetRawTransaction(hash)
	tx := MsgTxToTx(txData.MsgTx(), btcNet)
	return &tx, nil
}

func (c *Client) GetSpendableOutputs(params common.TxQueryParams, testNet bool, optionalMinMaxConfs ...int) ([]common.Transaction, error) {
	minConfs, maxConfs := int(0), 9999999
	if 0 < len(optionalMinMaxConfs) {
		if 2 < len(optionalMinMaxConfs) || len(optionalMinMaxConfs) == 1 {
			return nil, fmt.Errorf("expected 0 or 2 items in optionalMinMaxConfs but got %d", len(optionalMinMaxConfs))
		}
		minConfs, maxConfs = optionalMinMaxConfs[0], optionalMinMaxConfs[1]
	}
	btcNet := &chaincfg.MainNetParams
	if testNet {
		btcNet = &chaincfg.TestNet3Params
	}
	addr, err := btcutil.DecodeAddress(params.Address, btcNet)
	if err != nil {
		return nil, err
	}
	list, err := c.ListUnspentMinMaxAddresses(minConfs, maxConfs, []btcutil.Address{addr})
	if err != nil {
		return nil, err
	}
	log.Debugf("listunspent response: %+v", list)
	txs := make([]common.Transaction, 0, len(list))
	for _, unspent := range list {
		if unspent.Address != params.Address {
			continue
		}
		amount, err := common.NewAmountFromString(unspent.Amount.String())
		if err != nil {
			log.Info(err)
		}
		tx := common.Transaction{
			TxID:          unspent.TxID,
			To:            unspent.Address,
			Amount:        amount,
			Currency:      common.BTC,
			Confirmations: unspent.Confirmations,
			OutputIndex:   int(unspent.Vout),
			Spent:         false,
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (c *Client) btcTxsToCmnTransactions(params common.TxQueryParams, list map[string]*btcjson.GetTransactionResult, testNet bool) ([]common.Transaction, error) {
	txs := make([]common.Transaction, 0, len(list))
	for _, res := range list {
		if len(res.Details) == 0 ||
			(0 < params.TimeFrom && res.BlockTime < params.TimeFrom) ||
			(0 < params.TimeTo && params.TimeTo < res.BlockTime) {
			continue
		}
		isSend := false
		for _, details := range res.Details {
			if !details.InvolvesWatchOnly {
				continue
			}
			if details.Category == TxTypeSend {
				isSend = true
				break
			}
		}
		log.Debugf("BTC TX %s time: %d, isSend=%v params=%+v", res.TxID, res.BlockTime, isSend, params)

		if isSend && params.Type != TxTypeSend {
			continue
		}
		if !isSend && params.Type != TxTypeReceived {
			continue
		}
		for _, details := range res.Details {
			fromAddr := params.Address
			value, err := decimal.NewFromString(details.Amount.String())
			if err != nil {
				log.Info(err)
			}
			amount, err := common.NewAmountFromString(value.Abs().String())
			if err != nil {
				log.Info(err)
			}
			// ignore change sends, ignore non-send category
			if isSend {
				if details.Address == params.Address ||
					details.Category != TxTypeSend {
					continue
				}
			}
			if !isSend {
				if details.Address != params.Address ||
					details.Category != TxTypeReceived {
					continue
				}
				vinAddr, err := c.getFirstVinAddr(res.TxID, testNet)
				if err == nil {
					fromAddr = vinAddr
				}
			}
			txTime := res.BlockTime
			if txTime == 0 {
				txTime = res.Time
			}
			tx := common.Transaction{
				TxID:          res.TxID,
				To:            details.Address,
				From:          fromAddr,
				Amount:        amount,
				Currency:      common.BTC,
				Timestamp:     time.Unix(txTime, 0),
				Confirmations: res.Confirmations,
				OutputIndex:   int(details.Vout),
			}
			txs = append(txs, tx)
		}
	}
	log.Infof("BTC TXs time filtered. before: %d, after: %d", len(list), len(txs))
	return txs, nil
}

func (c *Client) getFirstVinAddr(txID string, testNet bool) (string, error) {
	// Only support bticoind full node not purne
	rawTx, err := c.GetTxByTxID(txID, testNet)
	if err != nil {
		return "", err
	}
	inTx0, err := c.GetTxByTxID(rawTx.Vin[0].Txid, testNet)
	if err != nil {
		return "", err
	}
	target := inTx0.Vout[rawTx.Vin[0].Vout]
	addr := target.Addresses[0]
	return addr, nil
}