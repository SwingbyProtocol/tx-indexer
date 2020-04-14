package common

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/btcsuite/btcutil"
)

type (
	UnspentTransactions struct {
		Amount   int64
		IsSpent  bool
		Currency Symbol
	}

	Transaction struct {
		TxID          string
		From          string
		To            string
		Amount        int64
		Timestamp     time.Time
		Currency      Symbol
		Confirmations int64
		Memo          string
		OutputIndex   int
		Spent         bool
	}

	TxQueryParams struct {
		Address string `json:"address"`
		// Txid       string `json:"txid,omitempty"`
		Type       string `json:"type"`
		Mempool    bool   `json:"mempool"`
		HeightFrom int64  `json:"height_from,omitempty"`
		HeightTo   int64  `json:"height_to,omitempty"`
		TimeFrom   int64  `json:"time_from"`
		TimeTo     int64  `json:"time_to"`
	}

	BroadcastParams struct {
		ToAddr string `json:"address"`
		Amount string `json:"amount"`
	}

	SwapSideConfig struct {
		Address, Currency                 string
		Fees                              SwapFeeParams
		ShouldCheckKVStoreWhenDestination bool
		MemoValidatorWhenDestination      func(string) bool
		AmountLimit                       int64
	}

	// TODO: move to fees.go?
	SwapFeeParams struct {
		FeePercent  float64
		FixedOutFee int64
	}

	SwapOutput struct {
		Address        string
		Amount         int64
		IsRewardPayout bool
		InTxId         string
		InCurrency     string
		Nonce          int
	}

	PlatformStatusResponse struct {
		Status int `json:"status"`
	}

	OtherChainAddr = btcutil.Address
)

func (tx Transaction) Serialize() string {
	return fmt.Sprintf("%s;%d;", tx.TxID, tx.OutputIndex)
}

func (tx Transaction) MarshalJSON() ([]byte, error) {
	newTx := struct {
		TxID          string `json:"txID"`
		From          string `json:"from"`
		To            string `json:"to"`
		Amount        int64  `json:"amount"`
		Timestamp     int64  `json:"time"`
		Currency      string `json:"currency"`
		Confirmations int64  `json:"confirmations"`
		Memo          string `json:"memo"`
		OutputIndex   int    `json:"outputIndex"`
		Spent         bool   `json:"spent"`
	}{
		TxID:          tx.TxID,
		From:          tx.From,
		To:            tx.To,
		Amount:        tx.Amount,
		Timestamp:     tx.Timestamp.Unix(),
		Currency:      tx.Currency.String(),
		Confirmations: tx.Confirmations,
		Memo:          tx.Memo,
		OutputIndex:   tx.OutputIndex,
		Spent:         tx.Spent,
	}
	return json.Marshal(newTx)
}
