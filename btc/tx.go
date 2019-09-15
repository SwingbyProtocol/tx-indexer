package btc

type Tx struct {
	Txid     string  `json:"txid"`
	Hash     string  `json:"hash"`
	Confirms int64   `json:"confirms"`
	Version  int     `json:"version"`
	Weight   int     `json:"weight"`
	Locktime int     `json:"locktime"`
	Vin      []*Vin  `json:"vin"`
	Vout     []*Vout `json:"vout"`
	//Hex      string  `json:"hex"`
}

type Vin struct {
	Txid     string `json:"txid"`
	Vout     int    `json:"vout"`
	Sequence int64  `json:"sequence"`
}

type Vout struct {
	Value        float32       `json:"value"`
	N            int           `json:"n"`
	ScriptPubkey *ScriptPubkey `json:"scriptPubkey"`
}

type ScriptPubkey struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	ReqSigs   int      `json:"reqSigs"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses"`
}
