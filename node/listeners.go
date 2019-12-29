package node

import (
	"errors"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

const (
	SFNodeBitcoinCash wire.ServiceFlag = 1 << 5
)

func (node *Node) OnVersion(p *peer.Peer, msg *wire.MsgVersion) *wire.MsgReject {
	remoteAddr := p.NA()
	remoteAddr.Services = msg.Services
	// Ignore peers that have a protcol version that is too old.  The peer
	// negotiation logic will disconnect it after this callback returns.
	if msg.ProtocolVersion < int32(peer.MinAcceptableProtocolVersion) {
		return nil
	}
	// Check the full node acceptance.
	isFullNode := remoteAddr.HasService(wire.SFNodeNetwork)
	isBCash := remoteAddr.HasService(SFNodeBitcoinCash)
	isSupportBloom := remoteAddr.HasService(wire.SFNodeBloom)
	isSegwit := remoteAddr.HasService(wire.SFNodeWitness)
	// Reject outbound peers that are not full nodes.
	if !isFullNode {
		log.Debugf("Peer %s is not full node", p)
		reason := "Peer is not full node"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	// Don't connect to bitcoin cash nodes
	if isBCash {
		log.Debugf("Peer %s does not support Bitcoin Cash", p)
		reason := "does not support Bitcoin Cash"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	if !(isSupportBloom && isSegwit) {
		// onDisconnection will be called
		// which will remove the peer from openPeers
		log.Debugf("Peer %s does not support bloom filtering and segwit, diconnecting...", p)
		reason := "Peer does not support bloom filtering and segwit"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	return nil
}

func (node *Node) OnVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	// Check this peer offers bloom filtering services. If not dump them.
	log.Debugf("Connected to %s - %s", p.Addr(), p.UserAgent())
}

func (node *Node) OnReject(p *peer.Peer, msg *wire.MsgReject) {
	log.Warningf("Received reject msg from peer %s: Code: %s, tx: %s, Reason: %s", p.Addr(), msg.Code.String(), msg.Hash.String()[:4], msg.Reason)
}

func (node *Node) OnInv(p *peer.Peer, msg *wire.MsgInv) {
	invVects := msg.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			iv := invVects[i]
			iv.Type = wire.InvTypeWitnessBlock
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
			continue
		}
		if invVects[i].Type == wire.InvTypeTx {
			//fmt.Println("inv_tx", invVects[i].Hash, p.Addr())
			iv := invVects[i]
			iv.Type = wire.InvTypeWitnessTx
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
			continue
		}
	}
}

func (node *Node) OnTx(p *peer.Peer, msg *wire.MsgTx) {
	//isWitness := msgTx.HasWitness()
	// Update node rank
	node.updateRank(p)
	// Get now ranks counts
	top, _, _, olders := node.GetRank()
	if top >= DefaultNodeRankSize {
		node.resetConnectedRank()
		// Remove end peers
		if len(olders) > 2 {
			for i := 0; i < 2; i++ {
				peer := node.GetConnectedPeer(olders[i])
				if peer != nil {
					peer.Disconnect()
				}
			}
		}
		// Finding new peer
		go node.queryDNSSeeds()
	}

	go func() {
		node.txChan <- msg
	}()
}

func (node *Node) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	go func() {
		node.BlockChan <- msg
	}()
}

func (node *Node) OnGetData(p *peer.Peer, msg *wire.MsgGetData) {
	for _, iv := range msg.InvList {
		var err error
		switch iv.Type {
		case wire.InvTypeWitnessTx:
			err = node.pushTxMsg(p, &iv.Hash, wire.WitnessEncoding)
		case wire.InvTypeTx:
			err = node.pushTxMsg(p, &iv.Hash, wire.BaseEncoding)
		default:
			log.Warnf("Unknown type in inventory request %d", iv.Type)
			continue
		}
		if err != nil {
			log.Info(err)
			continue
		}
		txid := iv.Hash.String()
		go func() {
			// Remove inv txs after time transition
			time.Sleep(30 * time.Second)
			err := node.RemoveInvTx(txid)
			if err == nil {
				log.Infof("Removed inv txs %s", txid)
			}
		}()
	}
}

func (node *Node) pushTxMsg(p *peer.Peer, hash *chainhash.Hash, enc wire.MessageEncoding) error {
	msgTx := node.GetInvTx(hash.String())
	if msgTx == nil {
		return errors.New("Unable to fetch tx from invtxs")
	}
	p.QueueMessageWithEncoding(msgTx, nil, enc)
	log.Infof("Broadcast tx data success: %s peer: %s", hash.String(), p.Addr())
	return nil
}

func (node *Node) sendBroadcastInv(iv *wire.InvVect) {
	peers := node.ConnectedPeers()
	for _, peer := range peers {
		peer.QueueInventory(iv)
	}
}
