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

type Transaction struct {
	TxID          string
	From          string
	To            string
	Amount        Amount
	Decimals      int
	Timestamp     time.Time
	Currency      Symbol
	Height        int64 // add height
	Confirmations int64
	Memo          string
	OutputIndex   int
	Spent         bool
}

func (tx Transaction) Serialize() string {
	return fmt.Sprintf("%s;%d;", tx.TxID, tx.OutputIndex)
}

func (tx Transaction) MarshalJSON() ([]byte, error) {
	newTx := struct {
		TxID          string      `json:"txId"`
		From          string      `json:"from"`
		To            string      `json:"to"`
		Amount        interface{} `json:"amount"`
		Decimals      int         `json:"decimals"`
		Timestamp     int64       `json:"time"`
		Height        int64       `json:"height"`
		Currency      string      `json:"currency"`
		Confirmations int64       `json:"confirmations"`
		Memo          string      `json:"memo"`
		OutputIndex   int         `json:"outputIndex"`
		Spent         bool        `json:"spent"`
	}{
		TxID:          tx.TxID,
		From:          tx.From,
		To:            tx.To,
		Amount:        tx.Amount.BigInt().String(),
		Decimals:      tx.Decimals,
		Timestamp:     tx.Timestamp.Unix(),
		Currency:      tx.Currency.String(),
		Height:        tx.Height,
		Confirmations: tx.Confirmations,
		Memo:          tx.Memo,
		OutputIndex:   tx.OutputIndex,
		Spent:         tx.Spent,
	}

	if newTx.Currency == ETH.String() {
		newTx.Amount = tx.Amount.BigInt().String()
	}
	return json.Marshal(newTx)
}
