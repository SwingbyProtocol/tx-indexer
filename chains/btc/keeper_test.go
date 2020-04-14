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

	k.SetAddr("mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9")

	k.Start()
}
