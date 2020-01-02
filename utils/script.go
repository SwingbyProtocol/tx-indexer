package utils

import (
	"encoding/hex"
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

type ScriptPubkeyInfo struct {
	Asm         string   `json:"asm"`
	Hex         string   `json:"hex"`
	Reqsigs     int      `json:"reqSigs"`
	ScriptClass string   `json:"type"`
	Addresses   []string `json:"addresses"`
}

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
