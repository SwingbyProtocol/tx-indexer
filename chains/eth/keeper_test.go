package eth

import (
	"os"
	"testing"
)

func TestKeeper(t *testing.T) {
	uri := os.Getenv("ethRPC")

	keeper := NewKeeper(uri, true)

	token := "0xaff4481d10270f50f203e0763e2597776068cbc5"

	tssAddr := "0x3Ec6671171710F13a1a980bc424672d873b38808"

	keeper.SetTokenAndWatchAddr(token, tssAddr)

	keeper.Start()

	select {}

}
