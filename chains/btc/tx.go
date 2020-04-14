package btc

import (
	"github.com/SwingbyProtocol/tx-indexer/utils"
)

type Tx struct {
	Txid         string  `json:"txid"`
	WitnessID    string  `json:"hash,omitempty"`
	Height       int64   `json:"height,omitempty"`
	Receivedtime int64   `json:"receivedtime,omitempty"`
	MinedTime    int64   `json:"minedtime,omitempty"`
	Mediantime   int64   `json:"mediantime,omitempty"`
	Version      int32   `json:"version,omitempty"`
	Weight       int64   `json:"weight,omitempty"`
	Locktime     uint32  `json:"locktime,omitempty"`
	Vin          []*Vin  `json:"vin,omitempty"`
	Vout         []*Vout `json:"vout,omitempty"`
}

type Vin struct {
	Txid      string      `json:"txid"`
	Coinbase  string      `json:"coinbase,omitempty"`
	Vout      uint32      `json:"vout"`
	Addresses []string    `json:"addresses"`
	Value     interface{} `json:"value"`
	Sequence  uint32      `json:"sequence"`
}

type Vout struct {
	Value        interface{}             `json:"value"`
	Spent        bool                    `json:"spent"`
	Txs          []string                `json:"txs"`
	Addresses    []string                `json:"addresses"`
	N            int                     `json:"n"`
	Scriptpubkey *utils.ScriptPubkeyInfo `json:"scriptPubkey"`
}

func (tx *Tx) GetTxID() string {
	return tx.Txid
}

func (tx *Tx) GetWitnessID() string {
	return tx.WitnessID
}

func (tx *Tx) AddBlockData(height int64, minedtime int64, medianTime int64) *Tx {
	tx.Height = height
	tx.MinedTime = minedtime
	tx.Mediantime = medianTime
	return tx
}

func (tx *Tx) GetOutsAddrs() []string {
	addresses := []string{}
	for _, vout := range tx.Vout {
		if len(vout.Scriptpubkey.Addresses) != 1 {
			// log.Debug("debug : len(vout.ScriptPubkey.Addresses) != 1")
			continue
		}
		addr := vout.Scriptpubkey.Addresses[0]
		if utils.CheckExist(addr, addresses) == true {
			continue
		}
		addresses = append(addresses, addr)
	}
	return addresses
}
