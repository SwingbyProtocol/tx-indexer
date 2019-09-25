package btc

import (
	"github.com/SwingbyProtocol/sc-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type Tx struct {
	Txid         string  `json:"txid"`
	Hash         string  `json:"hash"`
	Confirms     int64   `json:"confirms"`
	ReceivedTime int64   `json:"receivedtime"`
	MinedTime    int64   `json:"minedtime"`
	Mediantime   int64   `json:"mediantime"`
	Version      int     `json:"version"`
	Weight       int     `json:"weight"`
	Locktime     int     `json:"locktime"`
	Vin          []*Vin  `json:"vin"`
	Vout         []*Vout `json:"vout"`
	//Hex      string  `json:"hex"`
}

type Vin struct {
	Txid     string `json:"txid"`
	Vout     int    `json:"vout"`
	Sequence int64  `json:"sequence"`
}

type Vout struct {
	Value        interface{}   `json:"value"`
	Spent        bool          `json:"spent"`
	Txs          []string      `json:"txs"`
	N            int           `json:"n"`
	ScriptPubkey *ScriptPubkey `json:"scriptPubkey"`
}

type ScriptPubkey struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	ReqSigs   int      `json:"reqSigs"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses"`
}

func (tx *Tx) AddTxData(r *resolver.Resolver) error {
	newTx := Tx{}
	err := r.GetRequest("/rest/tx/"+tx.Txid+".json", &newTx)
	if err != nil {
		return err
	}
	tx.Hash = newTx.Hash
	tx.Version = newTx.Version
	tx.Weight = newTx.Weight
	tx.Locktime = newTx.Locktime
	tx.Vin = newTx.Vin
	tx.Vout = newTx.Vout
	return nil
}

func (tx *Tx) AddBlockData(block *Block) *Tx {
	tx.Confirms = block.Height
	tx.MinedTime = block.Time
	tx.Mediantime = block.Mediantime
	return tx
}

func (tx *Tx) getOutputsAddresses() []string {
	addresses := []string{}
	for _, vout := range tx.Vout {
		if len(vout.ScriptPubkey.Addresses) == 0 {
			log.Debug("debug : len(vout.ScriptPubkey.Addresses) == 0")
			continue
		}
		if len(vout.ScriptPubkey.Addresses) != 1 {
			log.Info("error : len(vout.ScriptPubkey.Addresses) != 1")
			continue
		}
		addresses = append(addresses, vout.ScriptPubkey.Addresses[0])
	}
	return addresses
}
