package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
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
		if req.Params == nil {
			c.SendJSON(api.CreateMsgErrorWS(req.Action, req.RequestID, "Params is not correct"))
			return
		}
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
		txins := []string{}
		for _, tx := range txs {
			for _, in := range tx.Vin {
				if in.Coinbase != "" {
					in.Addresses = []string{}
					in.Value = 0
				}
				if in.Value == nil && in.Coinbase == "" {
					txins = append(txins, in.Txid)
				}
			}
			for _, vout := range tx.Vout {
				if vout.Addresses == nil {
					vout.Addresses = []string{"OP_RETURN"}
				}
				vout.Txs = []string{}
			}
		}
		// get tx from other full node
		loaded := make(map[string]*types.Tx)
		nodes := node.GetNodes()
		if len(nodes) != 0 {
			for _, txid := range txins {
				tx := getTx(nodes, txid, 0)
				loaded[txid] = tx
			}
			for _, tx := range txs {
				for _, in := range tx.Vin {
					if in.Value == nil && in.Coinbase == "" {
						tx := loaded[in.Txid]
						vout := tx.Vout[in.Vout]
						in.Value = utils.ValueSat(vout.Value)
						in.Addresses = vout.Scriptpubkey.Addresses
					}
				}
			}
		}
		res := api.CreateMsgSuccessWS(api.GETTX, req.RequestID, "Get tx success", bc.GetLatestBlock().Height, txs)
		c.SendJSON(res)
		log.Infof("Get txs for %50s with params type %10s txs %d", req.Params.Txid, req.Params.Type, len(res.Txs))
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
		// collect all of missed txins
		txins := []string{}
		for _, tx := range txs {
			for _, in := range tx.Vin {
				if in.Coinbase != "" {
					in.Addresses = []string{}
					in.Value = 0
				}
				if in.Value == nil && in.Coinbase == "" {
					txins = append(txins, in.Txid)
				}
			}
		}
		// get tx from other full node
		loaded := make(map[string]*types.Tx)
		nodes := node.GetNodes()
		if len(nodes) != 0 {
			for _, txid := range txins {
				tx := getTx(nodes, txid, 0)
				loaded[txid] = tx
			}
			for _, tx := range txs {
				for _, in := range tx.Vin {
					if in.Value == nil && in.Coinbase == "" {
						tx := loaded[in.Txid]
						vout := tx.Vout[in.Vout]
						in.Value = utils.ValueSat(vout.Value)
						in.Addresses = vout.Scriptpubkey.Addresses
					}
				}
			}
		}
		if req.Params.NoDetails {
			for i, tx := range txs {
				txs[i] = &types.Tx{Txid: tx.Txid}
			}
		}
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
	getTxFromNodes := func(txid string, nodes []string) (*types.Tx, error) {
		tx, _ := bc.GetTx(txid)
		isExternal := false
		if tx == nil {
			// get tx from other full node
			tx = getTx(nodes, txid, len(nodes)-1)
			if tx == nil {
				msg := "tx value is invalid " + txid
				return nil, errors.New(msg)
			}
			isExternal = true
		}
		for _, in := range tx.Vin {
			if in.Coinbase != "" {
				in.Addresses = []string{}
				in.Value = 0
			}
		}
		// get input tx
		for _, in := range tx.Vin {
			if in.Value == nil && in.Coinbase == "" {
				inTx := getTx(nodes, in.Txid, len(nodes)-1)
				if inTx == nil {
					log.Infof("input invalid -> %s", in.Txid)
					continue
				}
				vout := inTx.Vout[in.Vout]
				in.Value = utils.ValueSat(vout.Value)
				in.Addresses = vout.Scriptpubkey.Addresses
			}
		}

		if isExternal {
			for _, vout := range tx.Vout {
				vout.Value = utils.ValueSat(vout.Value)
				vout.Addresses = vout.Scriptpubkey.Addresses
				if vout.Addresses == nil {
					vout.Addresses = []string{"OP_RETURN"}
				}
				vout.Txs = []string{}
			}
			// disabled cache for tx.
			// bc.TxChan() <- tx
		}
		return tx, nil
	}

	apiConfig.Listeners.OnGetTx = func(w rest.ResponseWriter, r *rest.Request) {
		txid := r.PathParam("txid")
		nodes := node.GetNodes()
		if len(nodes) == 0 {
			rest.Error(w, "nodes is not exist", http.StatusInternalServerError)
			return
		}
		tx, err := getTxFromNodes(txid, nodes)
		if err != nil {
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		txs := []*types.Tx{}
		txs = append(txs, tx)
		w.WriteHeader(http.StatusOK)
		w.WriteJson(txs)
		log.Infof("rest api call [getTx] -> %s", txid)
	}

	apiConfig.Listeners.OnGetTxMulti = func(w rest.ResponseWriter, r *rest.Request) {
		txids := types.Txids{}
		err := r.DecodeJsonPayload(&txids)
		if err != nil {
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		txs := []*types.Tx{}
		nodes := node.GetNodes()
		if len(nodes) == 0 {
			rest.Error(w, "nodes is not exist", http.StatusInternalServerError)
			return
		}
		for _, txid := range txids.Txids {
			tx, err := getTxFromNodes(txid, nodes)
			if err != nil {
				log.Info(err)
				continue
			}
			txs = append(txs, tx)
		}
		w.WriteHeader(http.StatusOK)
		w.WriteJson(txs)
		log.Infof("rest api call [getTxs] -> %d", len(txs))
	}

	apiConfig.Listeners.OnBroadcast = func(w rest.ResponseWriter, r *rest.Request) {
		hex := types.Broadcast{}
		err := r.DecodeJsonPayload(&hex)
		if err != nil {
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		utilTx, err := node.DecodeToTx(hex.HEX)
		if err != nil {
			log.Info(err)
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Validate tx
		err = node.ValidateTx(utilTx)
		if err != nil {
			log.Info(err)
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
		w.WriteHeader(http.StatusOK)
		res := types.Response{}
		res.Txid = txHash
		res.Result = true
		w.WriteJson(res)
		log.Infof("rest api call [broadcasted] -> %s %s", hex.HEX, res.Txid)
	}

	apiConfig.Listeners.OnGetMempool = func(w rest.ResponseWriter, r *rest.Request) {
		txs := bc.GetMempool()
		w.WriteHeader(http.StatusOK)
		w.WriteJson(txs)
		log.Infof("rest api call [getMempool] -> %d", len(txs))
	}

	apiConfig.Listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	apiServer.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGSTOP)
	signal := <-c
	// Backup operation
	log.Info(signal)
}

func getTx(addrs []string, txid string, index int) *types.Tx {
	resolver := api.NewResolver("http://" + addrs[index])
	resolver.SetTimeout(5 * time.Second)
	txData := types.Tx{}
	log.Infof("addr -> %s index %d", addrs[index], index)
	err := resolver.GetRequest("/rest/tx/"+txid+".json", &txData)
	if err != nil {
		if index == 0 {
			return nil
		}
		index--
		return getTx(addrs, txid, index)
	}
	return &txData
}
