package blockchain

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common/config"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const WitnessScaleFactor = 4

func ScriptToPubkeyInfo(script []byte, params *chaincfg.Params) (ScriptPubkeyInfo, error) {
	scriptClass, addrs, reqSig, err := txscript.ExtractPkScriptAddrs(script, params)
	if err != nil {
		return ScriptPubkeyInfo{}, err
	}
	if len(addrs) == 0 {
		return ScriptPubkeyInfo{}, errors.New("unknown script")
	}
	addresses := []string{}
	for _, addr := range addrs {
		addresses = append(addresses, addr.String())
	}
	spi := ScriptPubkeyInfo{
		// TODO: enable asm
		Asm:         "",
		Hex:         hex.EncodeToString(script),
		Reqsigs:     reqSig,
		ScriptClass: scriptClass.String(),
		Addresses:   addresses,
	}
	return spi, nil
}

func GetTransactionWeight(msgTx *wire.MsgTx) int64 {
	baseSize := msgTx.SerializeSizeStripped()
	totalSize := msgTx.SerializeSize()
	// (baseSize * 3) + totalSize
	return int64((baseSize * (WitnessScaleFactor - 1)) + totalSize)
}

func MsgTxToTx(msgTx *wire.MsgTx) Tx {

	tx := Tx{
		Txid:         msgTx.TxHash().String(),
		WitnessID:    msgTx.WitnessHash().String(),
		Version:      msgTx.Version,
		Locktime:     msgTx.LockTime,
		Weight:       GetTransactionWeight(msgTx),
		Receivedtime: time.Now().Unix(),
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
		spi, _ := ScriptToPubkeyInfo(txout.PkScript, &config.Set.P2PConfig.Params)
		newVout := &Vout{
			Value:        txout.Value,
			Spent:        false,
			Txs:          []string{},
			N:            i,
			Scriptpubkey: &spi,
		}
		tx.Vout = append(tx.Vout, newVout)
	}
	return tx
}

func MsgBlockToBlock(msgBlock *wire.MsgBlock) Block {
	block := Block{
		Hash: msgBlock.BlockHash().String(),
	}
	for _, msgTx := range msgBlock.Transactions {
		tx := MsgTxToTx(msgTx)
		block.Txs = append(block.Txs, &tx)
	}

	return block
}
