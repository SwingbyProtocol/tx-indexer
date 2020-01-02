package api

import (
	"github.com/SwingbyProtocol/tx-indexer/api/pubsub"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/ant0ine/go-json-rest/rest"
)

type Listeners struct {
	// OnKeep is invoked when a peer receives a getaddr bitcoin message.
	OnKeep func(w rest.ResponseWriter, r *rest.Request)
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnGetAddressIndex func(w rest.ResponseWriter, r *rest.Request)
	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnGetTx func(w rest.ResponseWriter, r *rest.Request)
	// OnGetWatchTxWS is invoked when api get address watch messsage.
	OnWatchTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetWatchTxWS is invoked when api get address watch messsage.
	OnUnWatchTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetTx is invoked when api get tx request messsage.
	OnGetTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// OnGetTxs is invoked when api get txs request messsage.
	OnGetIndexTxsWS func(c *pubsub.Client, req *MsgWsReqest)
	// Broadcast
	OnBroadcastTxWS func(c *pubsub.Client, req *MsgWsReqest)
	// Publish is invoked when new tx is stored to index.
	Publish func(ps *pubsub.PubSub, tx *types.PushMsg)
}
