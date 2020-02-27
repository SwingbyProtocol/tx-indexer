package main

import (
	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	log "github.com/sirupsen/logrus"
)

const (
	Received = "received"
	Send     = "send"
)

func onWatchTxWS(c *api.Client, req *api.Request) {
	if req.Params == nil {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params is not correct"))
		return
	}
	params := req.Params.(api.MsgWsParams)
	if params.Address == "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Address is not correct"))
		return
	}
	if !(params.Type == "" || params.Type == Received || params.Type == Send) {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
		return
	}
	if !params.Mempool {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Mempool should be true to call watchTxs"))
		return
	}
	if params.Type == "" {
		params.Type = Received
	}
	topic := params.Type + "_" + params.Address
	apiServer.Ws.Pubsub.Subscribe(c, topic)
	log.Infof("new subscription registered for : %s when %s by %s", params.Address, params.Type, c.ID())

	msg := "watch success for " + params.Address + " when " + params.Type
	c.SendJSON(CreateMsgSuccessWS(req.Action, msg, bc.GetLatestBlock(), []*types.Tx{}))
}

func onUnWatchTxWS(c *api.Client, req *api.Request) {
	if req.Params == nil {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params is not correct"))
		return
	}
	params := req.Params.(api.MsgWsParams)
	if params.Address == "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Address is not correct"))
		return
	}
	if !(params.Type == "" || params.Type == Received || params.Type == Send) {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
		return
	}
	if params.Type == "" {
		params.Type = Received
	}
	topic := params.Type + "_" + params.Address
	apiServer.Ws.Pubsub.Unsubscribe(c, topic)
	log.Infof("Client want to unsubscribe the Address: -> %s %s", params.Address, c.ID)

	msg := "unwatch success for " + params.Address + " when " + params.Type
	c.SendJSON(CreateMsgSuccessWS(req.Action, msg, bc.GetLatestBlock(), []*types.Tx{}))
}

func onGetIndexTxsWS(c *api.Client, req *api.Request) {
	if req.Params == nil {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params is not correct"))
		return
	}
	params := req.Params.(api.MsgWsParams)
	if params.Address == "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Address is not correct"))
		return
	}
	if !(params.Type == "" || params.Type == Received || params.Type == Send) {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Type is not correct"))
		return
	}
	if params.Type == "" {
		params.Type = Received
	}
	//state := blockchain.Received
	//if params.Type == Send {
	//	state = blockchain.Send
	//}
	timeFrom := params.TimeFrom
	timeTo := params.TimeTo
	mempool := params.Mempool
	if mempool && (timeFrom != 0 || timeTo != 0) {
		c.SendJSON(CreateMsgErrorWS(req.Action, "time windows cannot enable mempool flag"))
		log.Warn("Get txs call: time windows cannot enable mempool flag")
		return
	}
	txs := bc.GetIndexTxsWithTW(params.Address, timeFrom, timeTo, 1, mempool)
	res := CreateMsgSuccessWS(req.Action, "Get txs success only for "+params.Type, bc.GetLatestBlock(), txs)
	c.SendJSON(res)
	log.Infof("Get txs for %50s with params from %11d to %11d type %10s mempool %6t txs %d", params.Address, timeFrom, timeTo, params.Type, mempool, len(res.Txs.([]string)))
}

func onBroadcastTxWS(c *api.Client, req *api.Request) {
	if req.Params == nil {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params is not correct"))
		return
	}
	params := req.Params.(api.MsgWsParams)
	if params.Address != "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Address should be null"))
		return
	}
	if params.Type != "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Type should be null"))
		return
	}
	if params.Hex == "" {
		c.SendJSON(CreateMsgErrorWS(req.Action, "Params.Hex is not correct"))
		return
	}
	utilTx, err := utils.DecodeToTx(params.Hex)
	if err != nil {
		log.Info(err)
		c.SendJSON(CreateMsgErrorWS(req.Action, err.Error()))
		return
	}
	// Validate tx
	err = utils.ValidateTx(utilTx)
	if err != nil {
		log.Info(err)
		c.SendJSON(CreateMsgErrorWS(req.Action, err.Error()))
		return
	}
	addr := c.Conn().RemoteAddr().String()
	log.Infof("client: %s hash: %s hex: %s", addr, utilTx.Hash().String(), params.Hex)
	msgTx := utilTx.MsgTx()
	// Push to bc
	//bc.TxChan <- msgTx
	// Add tx to inv
	txHash := utilTx.Hash().String()
	instance.AddInvMsgTx(txHash, msgTx)
	for _, in := range msgTx.TxIn {
		inTx, _ := bc.GetTx(in.PreviousOutPoint.Hash.String())
		if inTx == nil {
			log.Debug("tx is not exit")
			continue
		}
		// Add inTx to inv
		//peer.AddInvTx(inTx.Txid, inTx.MsgTx)
	}
	instance.BroadcastTxInv(txHash)
	res := CreateMsgSuccessWS(req.Action, "Tx data broadcast success: "+txHash, bc.GetLatestBlock(), []*types.Tx{})
	c.SendJSON(res)
}

func Publish(ps *api.PubSub, msg *types.PushMsg) {
	txs := []*types.Tx{}
	txs = append(txs, msg.Tx)
	if msg.State == Send {
		res := CreateMsgSuccessWS("broadcast", Send, bc.GetLatestBlock(), txs)
		ps.PublishJSON(Send+"_"+msg.Addr, res)
		return
	}
	if msg.State == Received {
		res := CreateMsgSuccessWS("broadcast", Received, bc.GetLatestBlock(), txs)
		ps.PublishJSON(Received+"_"+msg.Addr, res)
		return
	}
}

func CreateMsgSuccessWS(action string, message string, height int64, txs []*types.Tx) api.MsgWsResponse {
	msg := api.MsgWsResponse{
		Action:  action,
		Result:  true,
		Message: message,
		Height:  height,
		Txs:     txs,
	}
	return msg
}

func CreateMsgErrorWS(action string, errMsg string) api.MsgWsResponse {
	msg := api.MsgWsResponse{
		Action:  action,
		Result:  false,
		Message: errMsg,
		Txs:     []*types.Tx{},
	}
	return msg
}
