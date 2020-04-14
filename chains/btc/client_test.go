package btc

import (
	"os"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	log "github.com/sirupsen/logrus"
)

func TestClient(t *testing.T) {
	rpcPath := os.Getenv("RPC")
	if rpcPath == "" {
		return
	}
	//btcAddr := "mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9"
	txID := "35828eb878ca543d4e4c61fd5f4e706327d9107fefa7e90aa67561d84a2a111a"
	net := &chaincfg.TestNet3Params
	c, err := NewBtcClient(rpcPath)
	if err != nil {
		log.Info(err)
	}
	hash, _ := chainhash.NewHashFromStr(txID)
	// Node Start
	tx, err := c.GetTransaction(hash, true)
	if err != nil {
		log.Info(err)
	}
	if tx.TxID == "" {
		t.Fatalf("Expected txid to be '%s' but got '%s'", txID, tx.TxID)
	}
	tx2, err := c.GetRawTransaction(hash)
	if err != nil {
		log.Info(err)
	}
	newTx := MsgTxToTx(tx2.MsgTx(), net)
	for _, vout := range newTx.Vout {
		if len(vout.Addresses) == 0 {
			t.Fatalf("Expected count of vout.Addresses to be not '%d' but got '%d'", 0, len(vout.Addresses))
		}
	}
}
