package node

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestNode(t *testing.T) {

	config := &NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   38,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
		TrustedPeer:      "192.168.1.230:8333",
	}
	node := NewNode(config)

	node.Start()

	// t.Fatalf("Expected config.ListenAddr to be '%s' but got '%s'", testData, conf.P2PConfig.ConnAddr)

	select {}
}
