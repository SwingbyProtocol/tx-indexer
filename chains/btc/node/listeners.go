package node

import (
	"net"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/utils"
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
		log.Infof("Peer %s is not full node, diconnecting...", p)
		reason := "Peer is not full node"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	// Don't connect to bitcoin cash nodes
	if isBCash {
		log.Infof("Peer %s does not support Bitcoin Cash, diconnecting...", p)
		reason := "does not support Bitcoin Cash"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	if !isSegwit {
		log.Infof("Peer %s does not support segwit, diconnecting...", p)
		reason := "Peer does not support segwit"
		return wire.NewMsgReject(msg.Command(), wire.RejectNonstandard, reason)
	}
	if !isSupportBloom {
		log.Infof("Peer %s does not support bloom filtering, diconnecting...", p)
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
	txHash := msg.TxHash().String()
	if node.isReceived(txHash) {
		return
	}
	node.addReceived(txHash)
	tx := utils.MsgTxToTx(msg, node.peerConfig.ChainParams)
	go func() {
		node.txChan <- &tx
	}()
}

func (node *Node) onBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	block := utils.MsgBlockToBlock(msg, node.peerConfig.ChainParams)
	go func() {
		node.bChan <- &block
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

func (node *Node) onDisconneted(p *peer.Peer, conn net.Conn) {
	// Remove peers if peer conn is closed.
	node.mu.Lock()
	delete(node.connectedPeers, conn.RemoteAddr().String())
	node.mu.Unlock()
	log.Infof("Peer %s disconnected", p)
}

func (node *Node) addReceived(hash string) {
	node.mu.Lock()
	node.receivedTxs[hash] = true
	node.mu.Unlock()
	go func() {
		time.Sleep(4 * time.Minute)
		node.mu.Lock()
		delete(node.receivedTxs, hash)
		node.mu.Unlock()
	}()
}

func (node *Node) isReceived(hash string) bool {
	node.mu.RLock()
	defer node.mu.RUnlock()
	if node.receivedTxs[hash] {
		return true
	}
	return false
}
