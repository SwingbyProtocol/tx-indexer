package common

type Response struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg,omitempty"`
}

type TxResponse struct {
	Response
	LatestHeight  int64         `json:"latestHeight"`
	InTxsMempool  []Transaction `json:"inTxsMempool"`
	InTxs         []Transaction `json:"inTxs"`
	OutTxsMempool []Transaction `json:"outTxsMempool"`
	OutTxs        []Transaction `json:"outTxs"`
}
