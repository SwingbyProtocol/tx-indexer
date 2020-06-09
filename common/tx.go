package common

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type UnspentTransactions struct {
	Amount   int64
	IsSpent  bool
	Currency Symbol
}

type Txs []Transaction

func (txs Txs) GetRangeTxs(fromNum int, toNum int) Txs {
	rangeTxs := []Transaction{}
	for _, tx := range txs {
		if int64(fromNum) > tx.Height {
			continue
		}
		if int64(toNum) < tx.Height {
			continue
		}
		rangeTxs = append(rangeTxs, tx)
	}
	return rangeTxs
}

func (txs Txs) Send(address string) Txs {
	sendTxs := []Transaction{}
	for _, tx := range txs {
		if tx.From != address {
			continue
		}
		sendTxs = append(sendTxs, tx)
	}
	return sendTxs
}

func (txs Txs) Receive(address string) []Transaction {
	sendTxs := []Transaction{}
	for _, tx := range txs {
		if tx.To != address {
			continue
		}
		sendTxs = append(sendTxs, tx)
	}
	return sendTxs
}

func (txs Txs) SendMempool(address string) Txs {
	sendTxs := []Transaction{}
	for _, tx := range txs {
		if tx.From != address {
			continue
		}
		if tx.Height != 0 {
			continue
		}
		sendTxs = append(sendTxs, tx)
	}
	return sendTxs
}

func (txs Txs) ReceiveMempool(address string) []Transaction {
	sendTxs := []Transaction{}
	for _, tx := range txs {
		if tx.To != address {
			continue
		}
		if tx.Height != 0 {
			continue
		}
		sendTxs = append(sendTxs, tx)
	}
	return sendTxs
}

func (txs Txs) RemoveTxs(targetTime time.Time) []Transaction {
	newTxs := []Transaction{}
	for _, tx := range txs {
		if tx.Timestamp.Unix() < targetTime.Unix() {
			continue
		}
		newTxs = append(newTxs, tx)
	}
	log.Infof("Returns txs: %d removed: %d", len(newTxs), len(txs)-len(newTxs))
	return newTxs
}

type Transaction struct {
	TxID          string
	From          string
	To            string
	Amount        Amount
	Timestamp     time.Time
	Currency      Symbol
	Height        int64
	Confirmations int64
	Memo          string
	OutputIndex   int
	Spent         bool
}

type TransactionResponse struct {
	TxID          string `json:"txId"`
	From          string `json:"from"`
	To            string `json:"to"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	Decimals      int    `json:"decimals"`
	Height        int64  `json:"height"`
	Timestamp     int64  `json:"time"`
	Confirmations int64  `json:"confirmations"`
	Memo          string `json:"memo"`
	OutputIndex   int    `json:"outputIndex"`
	Spent         bool   `json:"spent"`
}

func (tx Transaction) Serialize() string {
	return fmt.Sprintf("%s;%d;", tx.TxID, tx.OutputIndex)
}

func (tx Transaction) MarshalJSON() ([]byte, error) {
	res := TransactionResponse{
		TxID:          tx.TxID,
		From:          tx.From,
		To:            tx.To,
		Amount:        tx.Amount.BigInt().String(),
		Currency:      tx.Currency.String(),
		Decimals:      tx.Currency.Decimlas(),
		Height:        tx.Height,
		Timestamp:     tx.Timestamp.Unix(),
		Confirmations: tx.Confirmations,
		Memo:          tx.Memo,
		OutputIndex:   tx.OutputIndex,
		Spent:         tx.Spent,
	}
	return json.Marshal(res)
}
