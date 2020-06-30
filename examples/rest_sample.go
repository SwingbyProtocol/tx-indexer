package main

import (
	"fmt"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/common"
)

var (
	apiEndpoint = "https://new-testnet-indexer.swingby.network/api/v1"
	watchAddr   = "2N9Rcb3Vz5g8Do51usJ8ywJ4oCZJ2RBs469"
)

type Res struct {
	common.Response
	InTxsMempool  []common.TxJSON `json:"inTxsMempool"`
	InTxs         []common.TxJSON `json:"inTxs"`
	OutTxsMempool []common.TxJSON `json:"outTxsMempool"`
	OutTxs        []common.TxJSON `json:"outTxs"`
}

func main() {
	api := api.NewResolver(apiEndpoint, 2)
	res := Res{}
	limit := "25"
loop:
	page := "1"
	err := api.GetRequest("/btc/txs?watch="+watchAddr+"&limit="+limit+"&page="+page, &res)
	if err != nil {
		fmt.Println(err)
	}
	for _, tx := range res.OutTxs {
		fmt.Println(tx)
	}
	time.Sleep(2 * time.Second)
	goto loop
}
