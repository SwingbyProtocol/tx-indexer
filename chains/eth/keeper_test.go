package eth

import (
	"os"
	"testing"
)

func TestKeeper(t *testing.T) {
	uri := os.Getenv("ethRPC")

	keeper := NewKeeper(uri, true)

	keeper.Start()

	select {}

}
