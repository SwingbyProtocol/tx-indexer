package types

import "encoding/json"

type Transaction struct {
	TxHash        string      `json:"txHash"`
	BlockHeight   int64       `json:"blockHeight"`
	TxType        string      `json:"txType"`
	Timestamp     int64       `json:"timeStamp"`
	TxFee         json.Number `json:"txFee"`
	TxAge         int64       `json:"txAge"`
	Code          int64       `json:"code"`
	Log           string      `json:"log"`
	ConfirmBlocks int         `json:"confirmBlocks"`
	Memo          string      `json:"memo"`
	Source        int64       `json:"source"`
	// for transfers
	FromAddr string      `json:"fromAddr"`
	ToAddr   string      `json:"toAddr"`
	Value    json.Number `json:"value"`
	TxAsset  string      `json:"txAsset"`
	// for multi-send txs
	HasChildren int `json:"hasChildren"`
	// for multi-send child txs, an exception case where the field name is `asset` instead of `txAsset`
	ChildTxAsset string `json:"asset"`
	OutputIndex  int    `json:"-"`
}

/*
	"txHash": "D2B366183D0F57BB51BAC7FE56BF9F57AA6012003846C933E5211F94B5722419",
	"blockHeight": 60666033,
	"txType": "TRANSFER",
	"timeStamp": 1579010160880,
	"fromAddr": "tbnb1gls26vjjqqjw7wfgs07707yr58lc8z8sxml4hk",
	"toAddr": "tbnb1egsdny4jd4snznpsef0c0g9h7y8hqp9gsfmg34",
	"value": 0.15049325,
	"txAsset": "BTC.B-918",
	"txFee": 0.00037500,
	"txAge": 99673,
	"code": 0,
	"log": "Msg 0: ",
	"confirmBlocks": 0,
	"memo": "tb1qr50lnf829my2r0yrsps0wd462uf60zfk2sal0q",
	"source": 0,
	"hasChildren": 0
*/
