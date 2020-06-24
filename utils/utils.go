package utils

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/rand"
	"sort"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
)

const (
	WitnessScaleFactor = 4
	coinbaseTx         = "0000000000000000000000000000000000000000000000000000000000000000"
)

func RandRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

func SortTx(txs []common.Transaction) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Serialize() < txs[j].Serialize() })
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Timestamp.UnixNano() < txs[j].Timestamp.UnixNano() })
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

func ValueSat(value interface{}) float64 {
	x := decimal.NewFromFloat(value.(float64))
	y := decimal.NewFromFloat(100000000)
	sat, _ := x.Mul(y).Float64()
	return sat
}

func MsgTxToTx(msgTx *wire.MsgTx, params *chaincfg.Params) types.Tx {
	tx := types.Tx{
		Txid:         msgTx.TxHash().String(),
		WitnessID:    msgTx.WitnessHash().String(),
		Version:      msgTx.Version,
		Locktime:     msgTx.LockTime,
		Weight:       GetTransactionWeight(msgTx),
		Receivedtime: time.Now(),
	}
	for _, txin := range msgTx.TxIn {
		newVin := &types.Vin{
			Txid:      txin.PreviousOutPoint.Hash.String(),
			Vout:      txin.PreviousOutPoint.Index,
			Addresses: []string{},
			Sequence:  txin.Sequence,
		}
		if newVin.Txid == coinbaseTx {
			newVin.Addresses = []string{"coinbase"}
		}
		tx.Vin = append(tx.Vin, newVin)
	}
	for i, txout := range msgTx.TxOut {
		spi, _ := ScriptToPubkeyInfo(txout.PkScript, params)
		newVout := &types.Vout{
			Value:        txout.Value,
			Spent:        false,
			Txs:          []string{},
			Addresses:    spi.Addresses,
			N:            i,
			Scriptpubkey: &spi,
		}
		tx.Vout = append(tx.Vout, newVout)
	}
	return tx
}

func ScriptToPubkeyInfo(script []byte, params *chaincfg.Params) (types.ScriptPubkeyInfo, error) {
	scriptClass, addrs, reqSig, err := txscript.ExtractPkScriptAddrs(script, params)
	if err != nil {
		return types.ScriptPubkeyInfo{}, err
	}
	if len(addrs) == 0 {
		return types.ScriptPubkeyInfo{}, errors.New("unknown script")
	}
	addresses := []string{}
	for _, addr := range addrs {
		addresses = append(addresses, addr.String())
	}
	spi := types.ScriptPubkeyInfo{
		// TODO: enable asm
		Asm:         "",
		Hex:         hex.EncodeToString(script),
		Reqsigs:     reqSig,
		ScriptClass: scriptClass.String(),
		Addresses:   addresses,
	}
	return spi, nil
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

func DecodeToTx(hexStr string) (*wire.MsgTx, error) {
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
	return &msgTx, nil
}
