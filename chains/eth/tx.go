package eth

type Tx struct {
	Hash        string `json:"hash"`
	BlockHash   string `json:"blockHash"`
	BlockNumber string `json:"blockNumber"`
	From        string `json:"from"`
	To          string `json:"to"`
	Gas         string `json:"gas"`
	GasPrice    string `json:"gasPrice"`
	Data        string `json:"data"`
}
