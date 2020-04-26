package btc

import (
	"os"
	"testing"
)

func TestKeeper(t *testing.T) {
	rpcPath := os.Getenv("RPC")
	if rpcPath == "" {
		return
	}
	k := NewKeeper(rpcPath, true)

	//timestamp := time.Now().Add(-20 * time.Hour)

	k.SetWatchAddr("mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9")

	k.Start()
}
