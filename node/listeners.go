package node

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

const (
	SFNodeBitcoinCash wire.ServiceFlag = 1 << 5
)

func (node *Node) onVersion(p *peer.Peer, msg *wire.MsgVersion) *wire.MsgReject {
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
		log.Debugf("Peer %s is not full node, diconnecting...", p)
		reason := "Peer is not full node"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	// Don't connect to bitcoin cash nodes
	if isBCash {
		log.Debugf("Peer %s does not support Bitcoin Cash, diconnecting...", p)
		reason := "does not support Bitcoin Cash"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	if !isSegwit {
		log.Debugf("Peer %s does not support segwit, diconnecting...", p)
		reason := "Peer does not support segwit"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	if !isSupportBloom {
		log.Debugf("Peer %s does not support bloom filtering, diconnecting...", p)
		reason := "Peer does not support bloom filtering"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	return nil
}

func (node *Node) onVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	// Check this peer offers bloom filtering services. If not dump them.
	log.Debugf("Connected to %s - %s", p.Addr(), p.UserAgent())
}

func (node *Node) onReject(p *peer.Peer, msg *wire.MsgReject) {
	//go node.queryDNSSeeds()
	log.Warningf("Received reject msg from peer %s: Code: %s, tx: %s, Reason: %s", p.Addr(), msg.Code.String(), msg.Hash.String()[:4], msg.Reason)
}

func (node *Node) onInv(p *peer.Peer, msg *wire.MsgInv) {
	invVects := msg.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		iv := invVects[i]
		if iv.Type == wire.InvTypeBlock {
			if p.IsWitnessEnabled() {
				iv.Type = wire.InvTypeWitnessBlock
			}
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
		}
		if iv.Type == wire.InvTypeTx {
			if p.IsWitnessEnabled() {
				iv.Type = wire.InvTypeWitnessTx
			}
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
		}
	}
}

func (node *Node) onTx(p *peer.Peer, msg *wire.MsgTx) {
	// Update node rank
	node.updateRank(p)
	// Get now ranks counts
	top, _, _, olders := node.GetRank()
	if top >= DefaultNodeRankSize {
		node.resetConnectedRank()
		// Remove end peers
		if len(olders) > 2 {
			for i := 0; i < 2; i++ {
				peer := node.ConnectedPeer(olders[i])
				if peer != nil {
					peer.Disconnect()
				}
			}
		}
		// Finding new peer
		go node.queryDNSSeeds()
	}

	tx := types.MsgTxToTx(msg, node.peerConfig.ChainParams)
	go func() {
		node.txChan <- &tx
	}()
}

func (node *Node) onBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	go func() {
		node.blockChan <- msg
	}()
}

func (node *Node) onGetData(p *peer.Peer, msg *wire.MsgGetData) {
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
			log.Debug(err)
			continue
		}
		txid := iv.Hash.String()
		go func() {
			// Remove inv txs after time transition
			time.Sleep(30 * time.Second)
			err := node.RemoveInvTx(txid)
			if err == nil {
				log.Infof("Remove inv txs %s", txid)
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
	log.Infof("Broadcast success txhash: %s peer: %s", hash.String(), p.Addr())
	return nil
}

func (node *Node) sendBroadcastInv(iv *wire.InvVect) {
	peers := node.ConnectedPeers()
	if len(peers) > 4 {
		peers = peers[:3]
	}
	for _, peer := range peers {
		peer.QueueInventory(iv)
	}
}
