package types

import (
	"time"
)

type Tx struct {
	Txid         string    `json:"txid"`
	WitnessID    string    `json:"hash,omitempty"`
	Height       int64     `json:"height,omitempty"`
	Receivedtime time.Time `json:"receivedtime,omitempty"`
	MinedTime    time.Time `json:"minedtime,omitempty"`
	Mediantime   time.Time `json:"mediantime,omitempty"`
	Version      int32     `json:"version,omitempty"`
	Weight       int64     `json:"weight,omitempty"`
	Locktime     uint32    `json:"locktime,omitempty"`
	Vin          []*Vin    `json:"vin,omitempty"`
	Vout         []*Vout   `json:"vout,omitempty"`
}

type Vin struct {
	Txid      string   `json:"txid"`
	Vout      uint32   `json:"vout"`
	Addresses []string `json:"addresses"`
	Value     int64    `json:"value"`
	Sequence  uint32   `json:"sequence"`
}

type Vout struct {
	Value        int64             `json:"value"`
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

func (tx *Tx) AddBlockData(height int64, minedtime time.Time, medianTime time.Time) *Tx {
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
		if checkExist(addr, addresses) == true {
			continue
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func checkExist(key string, array []string) bool {
	isexist := false
	for _, id := range array {
		if id == key {
			isexist = true
		}
	}
	return isexist
}
