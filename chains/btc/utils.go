package btc

import (
	"fmt"
	"strings"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

func MsgTxToTx(msgTx *wire.MsgTx, params *chaincfg.Params) Tx {
	tx := Tx{
		Txid:         msgTx.TxHash().String(),
		WitnessID:    msgTx.WitnessHash().String(),
		Version:      msgTx.Version,
		Locktime:     msgTx.LockTime,
		Weight:       utils.GetTransactionWeight(msgTx),
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
		spi, _ := utils.ScriptToPubkeyInfo(txout.PkScript, params)
		newVout := &Vout{
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

func btcTransactionsToChainTransactions(curHeight int64, txs []Tx, timeFromUnix, timeToUnix int64) ([]common.Transaction, error) {
	newTxs := make([]common.Transaction, 0, 64)
	for _, tx := range txs {
		if len(tx.Vin) == 0 {
			continue
		}
		// try our best to figure out what the applicable vIn address is
		fundingVIn := tx.Vin[0]
		for _, vIn := range tx.Vin {
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
			//val, err := common.NewAmountFromInt(vOut.Value)
			//if err != nil {
			//	log.Logger.Warningf("TX error decoding output amount: %s, %s", vOut.Value, err)
			//	continue
			//}
			//if len(vOut.Addresses) > 1 {
			//	log.Logger.Warningf("TX %s output contains more than one address: %v. only address 0 will be used.",
			//		tx.TxId, vOut)
			//}
			// TODO: this is best effort for now; improve it later
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
			log.Debugf("TX %s confirms: %d", tx.Txid, confirms)
			newTxs = append(newTxs, common.Transaction{
				TxID:          strings.ToLower(tx.Txid),
				From:          fundingVIn.Addresses[0], // TODO: may be multiple addresses for multisig transactions
				To:            vOut.Addresses[0],       // TODO: may be multiple addresses for multisig transactions
				Amount:        vOut.Value.(int64),
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
