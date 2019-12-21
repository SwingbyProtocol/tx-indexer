package common

import (
	"encoding/hex"
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func ScriptToPubkeyInfo(script []byte, params *chaincfg.Params) (ScriptPubkeyInfo, error) {
	scriptClass, addrs, reqSig, err := txscript.ExtractPkScriptAddrs(script, params)
	if err != nil {
		return ScriptPubkeyInfo{}, err
	}
	if len(addrs) == 0 {
		return ScriptPubkeyInfo{}, errors.New("unknown script")
	}
	spi := ScriptPubkeyInfo{
		// TODO: enable asm
		Asm:         "",
		Hex:         hex.EncodeToString(script),
		Reqsigs:     reqSig,
		ScriptClass: scriptClass.String(),
		Addresses:   addrs,
	}
	return spi, nil
}
