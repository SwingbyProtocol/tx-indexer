package types

type MempoolResponse struct {
	Result *Result `json:"result"`
}

type Result struct {
	Pending map[string]map[string]Tx `json:"pending"`
}
