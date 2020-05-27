package common

import (
	"encoding/json"
	"fmt"
	"time"
)

type UnspentTransactions struct {
	Amount   int64
	IsSpent  bool
	Currency Symbol
}

type Txs []Transaction

func (txs Txs) GetRangeTxs(fromNum int, toNum int) []Transaction {
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
