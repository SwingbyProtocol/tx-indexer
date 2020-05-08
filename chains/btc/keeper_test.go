package btc

import (
	"os"
	"testing"
	"time"
)

func TestKeeper(t *testing.T) {
	rpcPath := os.Getenv("RPC")
	if rpcPath == "" {
		return
	}
	k := NewKeeper(rpcPath, true, "access_token")

	timestamp := time.Now().Add(-20 * time.Hour)

	k.SetWatchAddr("mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9", true, timestamp.Unix())

	k.Start()
}
