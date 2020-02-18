package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

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

func init() {
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcname := s[len(s)-1]
			//_, filename := path.Split(f.File)
			paddedFuncname := fmt.Sprintf(" %-20v", funcname+"()")
			//paddedFilename := fmt.Sprintf("%17v", filename)
			return paddedFuncname, ""
		},
	})
	log.SetOutput(os.Stdout)
}

func main() {
	conf, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	loglevel := conf.NodeConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	// Create Config
	blockchianConfig := &blockchain.BlockchainConfig{
		TrustedNode: conf.RESTConfig.ConnAddr,
		PruneSize:   conf.NodeConfig.PurneSize,
	}
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

	onKeepWs := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Address is not correct"))
			return
		}
		if len(req.RequestID) > 8 {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "RequestID is invalid"))
			return
		}
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, req.RequestID, "keep success", bc.GetLatestBlock().Height, []*types.Tx{}))
	}

	onWatchTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Address is not correct"))
			return
		}
		if len(req.RequestID) > 8 {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "RequestID is invalid"))
			return
		}
		if !(req.Params.Type == "" || req.Params.Type == Received || req.Params.Type == Send) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Type is not correct"))
			return
		}
		if !req.Params.Mempool {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Mempool should be true to call watchTxs"))
			return
		}
		if req.Params.Type == "" {
			req.Params.Type = Received
		}
		topic := req.Params.Type + "_" + req.Params.Address
		apiServer.GetWs().GetPubsub().Subscribe(c, topic)
		log.Infof("new subscription registered for : %s when %s by %s", req.Params.Address, req.Params.Type, c.ID)

		msg := "watch success for " + req.Params.Address + " when " + req.Params.Type
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, req.RequestID, msg, bc.GetLatestBlock().Height, []*types.Tx{}))
	}

	onUnWatchTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Address is not correct"))
			return
		}
		if len(req.RequestID) > 8 {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "RequestID is invalid"))
			return
		}
		if !(req.Params.Type == "" || req.Params.Type == Received || req.Params.Type == Send) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Type is not correct"))
			return
		}
		if req.Params.Type == "" {
			req.Params.Type = Received
		}
		topic := req.Params.Type + "_" + req.Params.Address
		apiServer.GetWs().GetPubsub().Unsubscribe(c, topic)
		log.Infof("Client want to unsubscribe the Address: -> %s %s", req.Params.Address, c.ID)

		msg := "unwatch success for " + req.Params.Address + " when " + req.Params.Type
		c.SendJSON(api.CreateMsgSuccessWS(req.Action, req.RequestID, msg, bc.GetLatestBlock().Height, []*types.Tx{}))
	}

	Publish := func(ps *pubsub.PubSub, msg *types.PushMsg) {
		txs := []*types.Tx{}
		txs = append(txs, msg.Tx)
		if msg.State == blockchain.Send {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, "", Send, bc.GetLatestBlock().Height, txs)
			ps.PublishJSON(Send+"_"+msg.Addr, res)
			return
		}
		if msg.State == blockchain.Received {
			res := api.CreateMsgSuccessWS(api.WATCHTXS, "", Received, bc.GetLatestBlock().Height, txs)
			ps.PublishJSON(Received+"_"+msg.Addr, res)
			return
		}
	}

	onGetTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params.Txid == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Params.Txid, req.RequestID, "txid is not set"))
			return
		}
		tx, _ := bc.GetTx(req.Params.Txid)
		if tx == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "tx is not exist"))
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
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
		if req.Params.Address == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Address is not correct"))
			return
		}
		if len(req.RequestID) > 8 {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "RequestID is invalid"))
			return
		}
		if !(req.Params.Type == "" || req.Params.Type == Received || req.Params.Type == Send) {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Type is not correct"))
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
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "time windows cannot enable mempool flag"))
			log.Warn("Get txs call: time windows cannot enable mempool flag")
			return
		}
		txs := bc.GetIndexTxsWithTW(req.Params.Address, timeFrom, timeTo, state, mempool)
		res := api.CreateMsgSuccessWS(api.GETTXS, req.RequestID, "Get txs success only for "+req.Params.Type, bc.GetLatestBlock().Height, txs)
		c.SendJSON(res)
		log.Infof("Get txs for %50s with params from %11d to %11d type %10s mempool %6t txs %d", req.Params.Address, timeFrom, timeTo, req.Params.Type, mempool, len(res.Txs))
	}

	onBroadcastTxWS := func(c *pubsub.Client, req *api.MsgWsReqest) {
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
		if req.Params.Address != "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Address should be null"))
			return
		}
		if len(req.RequestID) > 8 {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "RequestID is invalid"))
			return
		}
		if req.Params.Type != "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Type should be null"))
			return
		}
		if req.Params.Hex == "" {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params.Hex is not correct"))
			return
		}
		utilTx, err := node.DecodeToTx(req.Params.Hex)
		if err != nil {
			log.Info(err)
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, err.Error()))
			return
		}
		// Validate tx
		err = node.ValidateTx(utilTx)
		if err != nil {
			log.Info(err)
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, err.Error()))
			return
		}
		addr := c.Connection.RemoteAddr().String()
		log.Infof("client: %s hash: %s hex: %s", addr, utilTx.Hash().String(), req.Params.Hex)
		msgTx := utilTx.MsgTx()
		// Add tx to store
		tx := types.MsgTxToTx(msgTx, conf.P2PConfig.Params)
		// Push to bc
		bc.TxChan() <- &tx
		// Add tx to inv
		txHash := utilTx.Hash().String()
		node.AddInvTx(txHash, msgTx)
		for _, in := range msgTx.TxIn {
			inTx, _ := bc.GetTx(in.PreviousOutPoint.Hash.String())
			if inTx == nil {
				log.Debug("tx is not exit")
				continue
			}
			// Add inTx to inv
			node.AddInvTx(inTx.Txid, inTx.MsgTx)
		}
		node.BroadcastTxInv(txHash)
		res := api.CreateMsgSuccessWS(api.BROADCAST, req.RequestID, "Tx data broadcast success: "+txHash, bc.GetLatestBlock().Height, []*types.Tx{})
		c.SendJSON(res)
	}
	// Add handler for WS realtime
	apiConfig.Listeners.OnWatchTxWS = onWatchTxWS
	apiConfig.Listeners.OnUnWatchTxWS = onUnWatchTxWS
	apiConfig.Listeners.Publish = Publish
	// Add handler for WS
	apiConfig.Listeners.OnKeepWS = onKeepWs
	apiConfig.Listeners.OnGetTxWS = onGetTxWS
	apiConfig.Listeners.OnGetIndexTxsWS = onGetIndexTxsWS
	apiConfig.Listeners.OnBroadcastTxWS = onBroadcastTxWS
	// Add handler for REST
	apiConfig.Listeners.OnGetTx = bc.OnGetTx
	apiConfig.Listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	apiServer.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGSTOP)
	signal := <-c
	// Backup operation
	err = bc.Backup()
	if err != nil {
		log.Error(err)
	}
	log.Info(signal)
}
