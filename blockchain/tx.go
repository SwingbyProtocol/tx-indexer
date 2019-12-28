package blockchain

import (
	"github.com/SwingbyProtocol/tx-indexer/common"
)

type Tx struct {
	Txid         string  `json:"txid"`
	WitnessID    string  `json:"hash"`
	Confirms     int64   `json:"confirms"`
	Receivedtime int64   `json:"receivedtime"`
	MinedTime    int64   `json:"minedtime"`
	Mediantime   int64   `json:"mediantime"`
	Version      int32   `json:"version"`
	Weight       int64   `json:"weight"`
	Locktime     uint32  `json:"locktime"`
	Vin          []*Vin  `json:"vin"`
	Vout         []*Vout `json:"vout"`
	//Hex      string  `json:"hex"`
}

type Vin struct {
	Txid      string      `json:"txid"`
	Vout      uint32      `json:"vout"`
	Addresses []string    `json:"addresses"`
	Value     interface{} `json:"value"`
	Sequence  uint32      `json:"sequence"`
}

type Vout struct {
	Value        interface{}       `json:"value"`
	Spent        bool              `json:"spent"`
	Txs          []string          `json:"txs"`
	Addresses    []string          `json:"addresses"`
	N            int               `json:"n"`
	Scriptpubkey *ScriptPubkeyInfo `json:"scriptPubkey"`
}

func (tx *Tx) GetTxID() string {
	return tx.Txid
}

func (tx *Tx) GetWitnessID() string {
	return tx.WitnessID
}

func (tx *Tx) AddBlockData(height int64, time int64, medianTime int64) *Tx {
	tx.Confirms = height
	tx.MinedTime = time
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
		if common.CheckExist(addr, addresses) == true {
			continue
		}
		addresses = append(addresses, addr)
	}
	return addresses
}
