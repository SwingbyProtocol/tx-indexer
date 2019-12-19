package node

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestNode(t *testing.T) {
	config := &NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   25,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
	}
	node := NewNode(config)

	node.Start()

	select {}
}
