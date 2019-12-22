package main

import (
	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/common/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/btcsuite/btcd/chaincfg"
	log "github.com/sirupsen/logrus"
)

func main() {
	_, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	// Create Config
	blockchianConfig := &blockchain.BlockchainConfig{}
	// Create blockchain instance
	blockchain := blockchain.NewBlockchain(blockchianConfig)
	// Start blockchain service
	blockchain.Start()

	nodeConfig := &node.NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   100,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
		// Add trusted P2P Node
		TrustedPeer: config.Set.P2PConfig.ConnAddr,
		TxChan:      blockchain.TxChan(),
		BlockChan:   blockchain.BlockChan(),
	}

	// Node initialize
	node := node.NewNode(nodeConfig)
	// Node Start
	node.Start()

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		RESTListen: config.Set.RESTConfig.ListenAddr,
		WSListen:   config.Set.WSConfig.ListenAddr,
		Listeners:  &api.Listeners{},
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)

	listeners := apiConfig.Listeners
	listeners.OnGetTxs = blockchain.OnGetTxs

	apiServer.Start()

	select {}

}
