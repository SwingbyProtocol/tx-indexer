package types

type TxQueryParams struct {
	Address string `json:"address"`
	// Txid       string `json:"txid,omitempty"`
	Type       string `json:"type"`
	Mempool    bool   `json:"mempool"`
	HeightFrom int64  `json:"height_from,omitempty"`
	HeightTo   int64  `json:"height_to,omitempty"`
	TimeFrom   int64  `json:"time_from"`
	TimeTo     int64  `json:"time_to"`
}
