package common

type BroadcastParams struct {
	HEX string `json:"hex"`
}

type BroadcastResponse struct {
	Response
	TxHash string `json:"txHash,omitempty"`
}
