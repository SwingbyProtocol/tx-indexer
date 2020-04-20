package utils

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const WitnessScaleFactor = 4

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
		Receivedtime: time.Now().Unix(),
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
		spi, _ := ScriptToPubkeyInfo(txout.PkScript, params)
		newVout := &types.Vout{
			Value:        float64(txout.Value),
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

func BtcTransactionsToChainTransactions(curHeight int64, txs []types.Tx, timeFromUnix, timeToUnix int64) ([]common.Transaction, error) {
	newTxs := make([]common.Transaction, 0, 64)
	for _, tx := range txs {
		if len(tx.Vin) == 0 {
			continue
		}
		// try our best to figure out what the applicable vIn address is
		fundingVIn := tx.Vin[0]
		for _, vIn := range tx.Vin {
			log.Info(vIn)
			if 0 < len(vIn.Addresses) && vIn.Addresses[0] != "not exist" {
				fundingVIn = vIn
				break
			}
		}
		if len(fundingVIn.Addresses) > 1 {
			log.Warningf("TX %s input 0 contains more than one address: %v. only address 0 will be used.",
				tx.Txid, fundingVIn.Addresses)
		}
		if len(fundingVIn.Addresses) == 0 || fundingVIn.Addresses[0] == "not exist" {
			return nil, fmt.Errorf("TX %s has no eligible funding vIn without a vIn of address 'not exist'", tx.Txid)
		}
		for idx, vOut := range tx.Vout {
			if len(vOut.Addresses) <= 0 {
				continue
			}
			// if the tx is just in the mempool `MinedTime` will be 0
			txTime := int64(0)
			if 0 < tx.MinedTime {
				txTime = tx.MinedTime
			}
			// don't blindly trust our source :)
			if (0 < timeFromUnix && txTime < timeFromUnix) ||
				(0 < timeToUnix && timeToUnix < txTime) {
				continue
			}

			for _, vIn := range tx.Vin {
				if vIn.Value == vOut.Value {
					fundingVIn = vIn
					break
				}
			}
			confirms := tx.Height
			if 0 < curHeight && 0 < tx.Height {
				confirms = curHeight - tx.Height
			}
			if confirms < 0 {
				return nil, fmt.Errorf("confirms < 0: %d. indexer height: %d", confirms, curHeight)
			}
			if 0 < confirms {
				confirms++ // count its including block as one confirmation
			}
			amount, _ := common.NewAmountFromFloat64(vOut.Value.(float64))

			log.Debugf("TX %s confirms: %d", tx.Txid, confirms)
			newTxs = append(newTxs, common.Transaction{
				TxID:          strings.ToLower(tx.Txid),
				From:          fundingVIn.Addresses[0], // TODO: may be multiple addresses for multisig transactions
				To:            vOut.Addresses[0],       // TODO: may be multiple addresses for multisig transactions
				Amount:        amount,
				Timestamp:     time.Unix(txTime, 0),
				Currency:      common.BTC,
				Confirmations: confirms,
				OutputIndex:   idx,
				Spent:         vOut.Spent,
			})
		}
	}
	return newTxs, nil
}
