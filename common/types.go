package common

import "github.com/btcsuite/btcutil"

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	Bestblockhash string `json:"bestblockhash"`
}

type ScriptPubkeyInfo struct {
	Asm         string `json:"asm"`
	Hex         string `json:"hex"`
	Reqsigs     int    `json:"reqSigs"`
	ScriptClass string `json:"type"`
	Addresses   []btcutil.Address
}
