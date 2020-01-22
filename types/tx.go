package types

import (
	"time"

	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

type Tx struct {
	Txid         string      `json:"txid"`
	WitnessID    string      `json:"hash"`
	Height       int64       `json:"height"`
	Receivedtime int64       `json:"receivedtime"`
	MinedTime    int64       `json:"minedtime"`
	Mediantime   int64       `json:"mediantime"`
	Version      int32       `json:"version"`
	Weight       int64       `json:"weight"`
	Locktime     uint32      `json:"locktime"`
	Vin          []*Vin      `json:"vin"`
	Vout         []*Vout     `json:"vout"`
	MsgTx        *wire.MsgTx `json:"-"`
}

type Vin struct {
	Txid      string      `json:"txid"`
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

type UTXOs struct {
	Utxos []*UTXO `json:"utxos"`
}

type UTXO struct {
	Height       int64                   `json:"height"`
	Value        interface{}             `json:"value"`
	Scriptpubkey *utils.ScriptPubkeyInfo `json:"scriptPubkey"`
}

func (tx *Tx) GetTxID() string {
	return tx.Txid
}

func (tx *Tx) GetWitnessID() string {
	return tx.WitnessID
}

func (tx *Tx) AddBlockData(height int64, minedtime int64, medianTime int64) *Tx {
	tx.Confirms = height
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

func MsgTxToTx(msgTx *wire.MsgTx, params *chaincfg.Params) Tx {
	tx := Tx{
		Txid:         msgTx.TxHash().String(),
		WitnessID:    msgTx.WitnessHash().String(),
		Version:      msgTx.Version,
		Locktime:     msgTx.LockTime,
		Weight:       utils.GetTransactionWeight(msgTx),
		Receivedtime: time.Now().Unix(),
		MsgTx:        msgTx,
	}

	for _, txin := range msgTx.TxIn {
		newVin := &Vin{
			Txid:     txin.PreviousOutPoint.Hash.String(),
			Vout:     txin.PreviousOutPoint.Index,
			Sequence: txin.Sequence,
		}
		tx.Vin = append(tx.Vin, newVin)
	}

	for i, txout := range msgTx.TxOut {
		// Ignore the error here because the sender could have used and exotic script
		// for his change and we don't want to fail in that case.
		spi, _ := utils.ScriptToPubkeyInfo(txout.PkScript, params)
		value := float64(txout.Value) / 100000000
		newVout := &Vout{
			Value:        value,
			Spent:        false,
			Txs:          []string{},
			N:            i,
			Scriptpubkey: &spi,
		}
		tx.Vout = append(tx.Vout, newVout)
	}
	return tx
}

func MsgBlockToBlock(msgBlock *wire.MsgBlock, params *chaincfg.Params) Block {
	block := Block{
		Hash: msgBlock.BlockHash().String(),
	}
	for _, msgTx := range msgBlock.Transactions {
		tx := MsgTxToTx(msgTx, params)
		block.Txs = append(block.Txs, &tx)
	}
	return block
}
