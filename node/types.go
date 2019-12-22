package node

import (
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
)

type NodeConfig struct {
	// The network parameters to use
	Params *chaincfg.Params
	// The target number of outbound peers. Defaults to 10.
	TargetOutbound uint32
	// UserAgentName specifies the user agent name to advertise.  It is
	// highly recommended to specify this value.
	UserAgentName string
	// UserAgentVersion specifies the user agent version to advertise.  It
	// is highly recommended to specify this value and that it follows the
	// form "major.minor.revision" e.g. "2.6.41".
	UserAgentVersion string
	// If this field is not nil the PeerManager will only connect to this address
	TrustedPeer string
	// Chan for Tx
	TxChan chan blockchain.Tx
	// Chan for Block
	BlockChan chan blockchain.Block
}
