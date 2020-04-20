package types

type TransactionsRes struct {
	Txs []Transaction `json:"txArray"`
}
type MultiSendTransactionsRes struct {
	Txs   []Transaction `json:"subTxDtoList"`
	Total int           `json:"totalNum"`
}
