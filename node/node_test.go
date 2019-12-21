package node

import (
	"fmt"
	"net"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestNode(t *testing.T) {

	trusted, err := net.ResolveTCPAddr("tcp", "192.168.1.230:8333")
	if err != nil {
		fmt.Print(err)
	}
	config := &NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   38,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
		TrustedPeer:      trusted,
	}
	node := NewNode(config)

	node.Start()

	select {}
}
