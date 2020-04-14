package btc

import (
	"testing"
	"time"
)

func TestNode(t *testing.T) {
	txChan := make(chan *Tx)
	BChan := make(chan *Block)

	nodeConfig := &NodeConfig{
		IsTestnet:        true,
		TargetOutbound:   25,
		UserAgentName:    "Tx-indexer",
		UserAgentVersion: "1.0.0",
		TrustedNode:      "192.168.1.1",
		TxChan:           txChan,
		BChan:            BChan,
	}
	// Node initialize
	node := NewNode(nodeConfig)
	// Node Start
	node.Start()
	// t.Fatalf("Expected config.ListenAddr to be '%s' but got '%s'", testData, conf.P2PConfig.ConnAddr)
	time.Sleep(2 * time.Second)
	if !node.start {
		t.Fatalf("Expected local peers count to be '%t' but got '%t'", true, node.start)
	}
	node.Stop()
	time.Sleep(10 * time.Second)
	node.Start()
	time.Sleep(3 * time.Second)
	node.Stop()
}
