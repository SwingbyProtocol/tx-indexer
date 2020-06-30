package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"
)

type Txs []Transaction

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

func (txs Txs) ReceiveMempool(address string) Txs {
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

func (txs Txs) Sort() Txs {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Serialize() < txs[j].Serialize() })
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Timestamp.UnixNano() > txs[j].Timestamp.UnixNano() })
	return txs
}

func (txs Txs) Page(page int, limit int) Txs {
	if len(txs) == 0 {
		return txs
	}
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 25
	}
	base := make(map[int]Txs)
	index := 1
	for count, tx := range txs {
		if count >= 1 && count%limit == 0 {
			index++
		}
		base[index] = append(base[index], tx)
	}
	if base[page] == nil {
		return Txs{}
	}
	return base[page]
}

type Transaction struct {
	TxID        string
	From        string
	To          string
	Amount      Amount
	Timestamp   time.Time
	Currency    Symbol
	Height      int64
	Memo        string
	OutputIndex int
	Spent       bool
}

type TxJSON struct {
	TxID        string `json:"txId"`
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Decimals    int    `json:"decimals"`
	Height      int64  `json:"height"`
	Timestamp   int64  `json:"time"`
	Memo        string `json:"memo"`
	OutputIndex int    `json:"outputIndex"`
	Spent       bool   `json:"spent"`
}

func (tx Transaction) Serialize() string {
	return fmt.Sprintf("%s;%d;", tx.TxID, tx.OutputIndex)
}

func (tx Transaction) MarshalJSON() ([]byte, error) {
	res := TxJSON{
		TxID:        tx.TxID,
		From:        tx.From,
		To:          tx.To,
		Amount:      tx.Amount.BigInt().String(),
		Currency:    tx.Currency.String(),
		Decimals:    tx.Currency.Decimlas(),
		Height:      tx.Height,
		Timestamp:   tx.Timestamp.Unix(),
		Memo:        tx.Memo,
		OutputIndex: tx.OutputIndex,
		Spent:       tx.Spent,
	}
	return json.Marshal(res)
}

func (t TxJSON) ToCommTx() (Transaction, error) {
	amt, result := new(big.Int).SetString(t.Amount, 0)
	if !result {
		return Transaction{}, errors.New("Error t.Amount can not convert to big.int")
	}
	amount, err := NewAmountFromBigInt(amt)
	if err != nil {
		return Transaction{}, err
	}
	tx := Transaction{
		TxID:        t.TxID,
		From:        t.From,
		To:          t.To,
		Amount:      amount,
		Currency:    NewSymbol(t.Currency, t.Decimals),
		Height:      t.Height,
		Timestamp:   time.Unix(t.Timestamp, 0),
		Memo:        t.Memo,
		OutputIndex: t.OutputIndex,
		Spent:       t.Spent,
	}
	return tx, nil
}
