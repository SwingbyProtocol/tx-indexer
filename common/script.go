package common

import (
	"encoding/hex"
	"errors"

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
