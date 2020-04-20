package common

type BroadcastParams struct {
	HEX string `json:"hex"`
}

type BroadcastResponse struct {
	TxHash string `json:"txHash,omitempty"`
	Result bool   `json:"result"`
	Msg    string `json:"msg,omitempty"`
}
