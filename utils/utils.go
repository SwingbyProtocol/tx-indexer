package utils

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const WitnessScaleFactor = 4

func RandRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

func GetMaxMin(ranks map[string]uint64) (uint64, uint64, string, []string) {
	top := uint64(0)
	min := uint64(^uint(0) >> 1)
	topAddr := ""
	olders := []string{}
	sorted := []string{}
	for addr, p := range ranks {
		if top < p {
			top = p
			topAddr = addr
		}
		if min > p {
			min = p
			if len(ranks) > 0 {
				olders = append(olders, addr)
			}
		}
	}
	for i := len(olders) - 1; i >= 0; i-- {
		sorted = append(sorted, olders[i])
	}
	return top, min, topAddr, sorted
}

func GetTransactionWeight(msgTx *wire.MsgTx) int64 {
	baseSize := msgTx.SerializeSizeStripped()
	totalSize := msgTx.SerializeSize()
	// (baseSize * 3) + totalSize
	return int64((baseSize * (WitnessScaleFactor - 1)) + totalSize)
}

func ValidateTx(tx *btcutil.Tx) error {
	err := CheckNonStandardTx(tx)
	if err != nil {
		return err
	}
	err = OutputRejector(tx.MsgTx())
	if err != nil {
		return err
	}
	return nil
}

func DecodeToTx(hexStr string) (*btcutil.Tx, error) {
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, err
	}
	tx := btcutil.NewTx(&msgTx)
	return tx, nil
}

func MsgTxToTx(msgTx *wire.MsgTx, params *chaincfg.Params) types.Tx {
	tx := types.Tx{
		Txid:         msgTx.TxHash().String(),
		WitnessID:    msgTx.WitnessHash().String(),
		Version:      msgTx.Version,
		Locktime:     msgTx.LockTime,
		Weight:       GetTransactionWeight(msgTx),
		Receivedtime: time.Now().Unix(),
		//MsgTx:        msgTx,
	}

	for _, txin := range msgTx.TxIn {
		newVin := &types.Vin{
			Txid:     txin.PreviousOutPoint.Hash.String(),
			Vout:     txin.PreviousOutPoint.Index,
			Sequence: txin.Sequence,
		}
		tx.Vin = append(tx.Vin, newVin)
	}

	for i, txout := range msgTx.TxOut {
		// Ignore the error here because the sender could have used and exotic script
		// for his change and we don't want to fail in that case.
		spi, _ := ScriptToPubkeyInfo(txout.PkScript, params)
		value := float64(txout.Value) / 100000000
		newVout := &types.Vout{
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

func MsgBlockToBlock(msgBlock *wire.MsgBlock, params *chaincfg.Params) types.Block {
	block := types.Block{
		Hash: msgBlock.BlockHash().String(),
	}
	for _, msgTx := range msgBlock.Transactions {
		tx := MsgTxToTx(msgTx, params)
		block.Txs = append(block.Txs, &tx)
	}
	return block
}
