package main

import (
	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
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
	blockchianConfig := &blockchain.BlockchainConfig{
		TrustedNode: config.Set.RESTConfig.ConnAddr,
	}
	// Create blockchain instance
	bc := blockchain.NewBlockchain(blockchianConfig)
	// Start blockchain service
	bc.Start()

	nodeConfig := &node.NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   20,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
		// Add trusted P2P Node
		TrustedPeer: config.Set.P2PConfig.ConnAddr,
		TxChan:      bc.TxChan(),
		BlockChan:   bc.BlockChan(),
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
		PushChan:   bc.PushTxChan(),
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)

	listeners := apiConfig.Listeners

	listeners.OnGetTx = bc.OnGetTx

	listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	listeners.OnWatchTxWS = func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params != nil && req.Params.Address != "" {
			apiServer.GetWs().GetPubsub().Subscribe(c, req.Params.Address)
			log.Infof("new subscription registered for : %s by %s", req.Params.Address, c.ID)
			c.SendJSON(api.CreateMsgSuccessWS(req.Action, "watch success", []string{}))
			return
		}
		c.SendJSON(api.CreateMsgErrorWS(req.Action, "Address is not set"))
	}

	listeners.OnUnWatchTxWS = func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params != nil && req.Params.Address != "" {
			apiServer.GetWs().GetPubsub().Unsubscribe(c, req.Params.Address)
			log.Infof("Client want to unsubscribe the Address: -> %s %s", req.Params.Address, c.ID)
			c.SendJSON(api.CreateMsgSuccessWS(req.Action, "unwatch success", []string{}))
			return
		}
		c.SendJSON(api.CreateMsgErrorWS(req.Action, "Address is not set"))
	}

	listeners.OnGetTxWS = func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params.Txid == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Params.Txid, "txid is not set"))
			return
		}
		tx, err := bc.GetTxScore().GetTx(req.Params.Txid)
		if err != nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, err.Error()))
			return
		}
		txs := []*blockchain.Tx{}
		txs = append(txs, tx)
		res := api.MsgWsResponse{
			Action:  req.Action,
			Result:  false,
			Message: "success",
			Txs:     txs,
		}
		c.SendJSON(res)
	}

	apiServer.Start()

	select {}

}
