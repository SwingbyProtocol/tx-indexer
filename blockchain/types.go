package blockchain

type BlockchainConfig struct {
	// TrustedNode is ip addr for connect to rest api
	TrustedNode string
}

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	Bestblockhash string `json:"bestblockhash"`
}

type ScriptPubkeyInfo struct {
	Asm         string   `json:"asm"`
	Hex         string   `json:"hex"`
	Reqsigs     int      `json:"reqSigs"`
	ScriptClass string   `json:"type"`
	Addresses   []string `json:"addresses"`
}

type Vin struct {
	Txid     string `json:"txid"`
	Vout     uint32 `json:"vout"`
	Sequence uint32 `json:"sequence"`
}

type Vout struct {
	Value        interface{}       `json:"value"`
	Spent        bool              `json:"spent"`
	Txs          []string          `json:"txs"`
	N            int               `json:"n"`
	Scriptpubkey *ScriptPubkeyInfo `json:"scriptPubkey"`
}

type Link struct {
}

type ErrorResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}
