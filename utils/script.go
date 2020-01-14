package utils

import (
	"encoding/hex"
	"errors"

	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

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
