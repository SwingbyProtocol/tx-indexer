package common

type Response struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg,omitempty"`
}

type TxResponse struct {
	Response
	InTxsMempool  []Transaction `json:"inTxsMempool"`
	InTxs         []Transaction `json:"inTxs"`
	OutTxsMempool []Transaction `json:"outTxsMempool"`
	OutTxs        []Transaction `json:"outTxs"`
}
