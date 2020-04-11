package api

type MsgWsParams struct {
	Address   string `json:"address"`
	Txid      string `json:"txid"`
	Hex       string `json:"hex"`
	Type      string `json:"type"`
	Mempool   bool   `json:"mempool"`
	NoDetails bool   `json:"nodetails"`
	TimeFrom  int64  `json:"time_from"`
	TimeTo    int64  `json:"time_to"`
}

type MsgWsResponse struct {
	Action  string      `json:"action"`
	Result  bool        `json:"result"`
	Height  int64       `json:"height"`
	Message string      `json:"message"`
	Txs     interface{} `json:"txs"`
}
