package common

import (
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

func ScriptToAddress(script []byte, params *chaincfg.Params) (btcutil.Address, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, params)
	if err != nil {
		return &btcutil.AddressPubKeyHash{}, err
	}
	if len(addrs) == 0 {
		return &btcutil.AddressPubKeyHash{}, errors.New("unknown script")
	}
	return addrs[0], nil
}
