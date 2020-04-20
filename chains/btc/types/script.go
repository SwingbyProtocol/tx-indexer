package types

type ScriptPubkeyInfo struct {
	Asm         string   `json:"asm"`
	Hex         string   `json:"hex"`
	Reqsigs     int      `json:"reqSigs"`
	ScriptClass string   `json:"type"`
	Addresses   []string `json:"addresses"`
}
