package types

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	Bestblockhash string `json:"bestblockhash"`
}

type PushMsg struct {
	Tx    *Tx
	Addr  string
	State int
}

type Response struct {
	Txid   string `json:"txid"`
	Result bool   `json:"result"`
}
