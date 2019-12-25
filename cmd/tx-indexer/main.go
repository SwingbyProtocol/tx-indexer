package main

import (
	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/common/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	log "github.com/sirupsen/logrus"
)

const (
	Received = "received"
	Send     = "send"
)

func main() {
	_, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	// Create Config
	blockchianConfig := &blockchain.BlockchainConfig{
		TrustedNode: config.Set.RESTConfig.ConnAddr,
		PruneSize:   config.Set.NodeConfig.PurneSize,
	}
	log.Infof("Start block syncing with pruneSize: %d", blockchianConfig.PruneSize)
	// Create blockchain instance
	bc := blockchain.NewBlockchain(blockchianConfig)
	// Start blockchain service
	bc.Start()

	nodeConfig := &node.NodeConfig{
		Params:           &config.Set.P2PConfig.Params,
		TargetOutbound:   config.Set.P2PConfig.TargetOutbound,
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
		RESTListen:  config.Set.RESTConfig.ListenAddr,
		WSListen:    config.Set.WSConfig.ListenAddr,
		Listeners:   &api.Listeners{},
		PushMsgChan: bc.PushMsgChan(),
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)

	listeners := apiConfig.Listeners

	listeners.OnGetTx = bc.OnGetTx

	listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	listeners.OnWatchTxWS = func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Address is not correct"))
			return
		}
		if !(req.Params.Type == "" || req.Params.Type == Received || req.Params.Type == Send) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
			return
		}
		if req.Params.Type == "" {
			req.Params.Type = Received
		}
		topic := req.Params.Type + "_" + req.Params.Address
		apiServer.GetWs().GetPubsub().Subscribe(c, topic)
		log.Infof("new subscription registered for : %s when %s by %s", req.Params.Address, req.Params.Type, c.ID)

		msg := "watch success for " + req.Params.Address + " when " + req.Params.Type
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, msg, []string{}))
	}

	listeners.OnUnWatchTxWS = func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Address is not correct"))
			return
		}
		if !(req.Params.Type == "" || req.Params.Type == Received || req.Params.Type == Send) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
			return
		}
		if req.Params.Type == "" {
			req.Params.Type = Received
		}
		topic := req.Params.Type + "_" + req.Params.Address
		apiServer.GetWs().GetPubsub().Unsubscribe(c, topic)
		log.Infof("Client want to unsubscribe the Address: -> %s %s", req.Params.Address, c.ID)

		msg := "unwatch success for " + req.Params.Address + " when " + req.Params.Type
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, msg, []string{}))
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

	listeners.Publish = func(ps *pubsub.PubSub, msg *blockchain.PushMsg) {
		txs := []*blockchain.Tx{}
		txs = append(txs, msg.Tx)
		if msg.State == blockchain.Send {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, Send, txs)
			ps.PublishJSON(Send+msg.Addr, res)
			return
		}
		if msg.State == blockchain.Received {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, Received, txs)
			ps.PublishJSON(Received+"_"+msg.Addr, res)
			return
		}
	}

	apiServer.Start()

	select {}

}
