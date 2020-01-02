package main

import (
	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/SwingbyProtocol/tx-indexer/types"
	log "github.com/sirupsen/logrus"
)

const (
	Received = "received"
	Send     = "send"
)

func main() {
	conf, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	// Create Config
	blockchianConfig := &blockchain.BlockchainConfig{
		TrustedNode: conf.RESTConfig.ConnAddr,
		PruneSize:   conf.NodeConfig.PurneSize,
	}
	log.Infof("Start block syncing with pruneSize: %d", blockchianConfig.PruneSize)
	log.Infof("Using trusted node: %s", conf.RESTConfig.ConnAddr)
	// Create blockchain instance
	bc := blockchain.NewBlockchain(blockchianConfig)
	// Start blockchain service
	bc.Start()

	nodeConfig := &node.NodeConfig{
		Params:           conf.P2PConfig.Params,
		TargetOutbound:   conf.P2PConfig.TargetOutbound,
		UserAgentName:    "Tx-indexer",
		UserAgentVersion: "1.0.0",
		TrustedPeer:      conf.P2PConfig.ConnAddr,
		TxChan:           bc.TxChan(),
		BlockChan:        bc.BlockChan(),
	}
	log.Infof("Using network -> %s", nodeConfig.Params.Name)
	// Node initialize
	node := node.NewNode(nodeConfig)
	// Node Start
	node.Start()

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		RESTListen:  conf.RESTConfig.ListenAddr,
		WSListen:    conf.WSConfig.ListenAddr,
		Listeners:   &api.Listeners{},
		PushMsgChan: bc.PushMsgChan(),
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)

	onWatchTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
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
		if !req.Params.Mempool {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params.Mempool should be true to call watchTxs"))
			return
		}
		if req.Params.Type == "" {
			req.Params.Type = Received
		}
		topic := req.Params.Type + "_" + req.Params.Address
		apiServer.GetWs().GetPubsub().Subscribe(c, topic)
		log.Infof("new subscription registered for : %s when %s by %s", req.Params.Address, req.Params.Type, c.ID)

		msg := "watch success for " + req.Params.Address + " when " + req.Params.Type
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, msg, []*types.Tx{}))
	}

	onUnWatchTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
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
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, msg, []*types.Tx{}))
	}

	onGetTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params.Txid == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Params.Txid, "txid is not set"))
			return
		}
		tx, _ := bc.GetTx(req.Params.Txid)
		if err == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, err.Error()))
			return
		}
		txs := []*types.Tx{}
		txs = append(txs, tx)
		res := api.MsgWsResponse{
			Action:  req.Action,
			Result:  false,
			Message: "success",
			Txs:     txs,
		}
		c.SendJSON(res)
	}

	onGetIndexTxsWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
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
		state := blockchain.Received
		if req.Params.Type == Send {
			state = blockchain.Send
		}
		timeFrom := req.Params.TimeFrom
		timeTo := req.Params.TimeTo
		mempool := req.Params.Mempool
		if mempool && (timeFrom != 0 || timeTo != 0) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "time windows cannot enable mempool flag"))
			log.Warn("Get txs call: time windows cannot enable mempool flag")
			return
		}
		txs, err := bc.GetIndexTxsWithTW(req.Params.Address, timeFrom, timeTo, state, mempool)
		if err != nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "txs is not correct"))
			return
		}
		res := api.CreateMsgSuccessWS(api.GETTXS, "Get txs success only for "+req.Params.Type, txs)
		c.SendJSON(res)
		log.Infof("Get txs for %s with params from %11d to %11d type %10s mempool %6t txs %d", req.Params.Address, timeFrom, timeTo, req.Params.Type, mempool, len(res.Txs))
	}

	Publish := func(ps *pubsub.PubSub, msg *types.PushMsg) {
		txs := []*types.Tx{}
		txs = append(txs, msg.Tx)
		if msg.State == blockchain.Send {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, Send, txs)
			ps.PublishJSON(Send+"_"+msg.Addr, res)
			return
		}
		if msg.State == blockchain.Received {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, Received, txs)
			ps.PublishJSON(Received+"_"+msg.Addr, res)
			return
		}
	}

	onBroadcastTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params is not correct"))
			return
		}
		if req.Params.Address != "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Address is not correct"))
			return
		}
		if req.Params.Type != "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
			return
		}
		if req.Params.Hex == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Params.Hex is not correct"))
			return
		}
		utilTx, err := node.DecodeToTx(req.Params.Hex)
		if err != nil {
			log.Info(err)
		}
		txHash := utilTx.Hash().String()
		msgTx := utilTx.MsgTx()
		// Add tx to store
		tx := types.MsgTxToTx(msgTx, conf.P2PConfig.Params)
		bc.TxChan() <- &tx
		// Add tx to inv
		node.AddInvTx(txHash, msgTx)
		for _, in := range msgTx.TxIn {
			inTx, _ := bc.GetTx(in.PreviousOutPoint.Hash.String())
			if inTx == nil {
				log.Info("tx is not exit")
				continue
			}
			// Add inTx to inv
			node.AddInvTx(inTx.Txid, inTx.MsgTx)
		}
		err = node.BroadcastTxInv(txHash)
		if err != nil {
			log.Info(err)
			c.SendJSON(api.CreateMsgErrorWS(req.Action, "Tx data is not correct"))
			return
		}
		res := api.CreateMsgSuccessWS(api.BROADCAST, "Tx data broadcast success: "+txHash, []*types.Tx{})
		c.SendJSON(res)
	}
	// Add handler for WS
	apiConfig.Listeners.OnWatchTxWS = onWatchTxWS
	apiConfig.Listeners.OnUnWatchTxWS = onUnWatchTxWS
	apiConfig.Listeners.OnGetTxWS = onGetTxWS
	apiConfig.Listeners.OnGetIndexTxsWS = onGetIndexTxsWS
	apiConfig.Listeners.OnBroadcastTxWS = onBroadcastTxWS
	apiConfig.Listeners.Publish = Publish
	// Add handler for REST
	apiConfig.Listeners.OnGetTx = bc.OnGetTx
	apiConfig.Listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	apiServer.Start()

	select {}

}
