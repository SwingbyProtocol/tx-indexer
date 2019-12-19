package node

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

var SFNodeBitcoinCash wire.ServiceFlag = 1 << 5

type NodeConfig struct {

	// The network parameters to use
	Params *chaincfg.Params

	// The target number of outbound peers. Defaults to 10.
	TargetOutbound uint32

	// UserAgentName specifies the user agent name to advertise.  It is
	// highly recommended to specify this value.
	UserAgentName string

	// UserAgentVersion specifies the user agent version to advertise.  It
	// is highly recommended to specify this value and that it follows the
	// form "major.minor.revision" e.g. "2.6.41".
	UserAgentVersion string

	// If this field is not nil the PeerManager will only connect to this address
	TrustedPeer net.Addr
}

type Node struct {
	chain          *BlockChain
	peerConfig     *peer.Config
	peerMutex      *sync.RWMutex
	targetOutbound uint32
	connectedPeers map[string]*peer.Peer
	txChan         chan Tx
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		peerMutex:      new(sync.RWMutex),
		targetOutbound: config.TargetOutbound,
		connectedPeers: make(map[string]*peer.Peer),
		txChan:         make(chan Tx),
	}

	listeners := &peer.MessageListeners{}
	listeners.OnVerAck = node.OnVerack
	listeners.OnAddr = node.OnAddr
	listeners.OnInv = node.OnInv
	listeners.OnTx = node.OnTx
	listeners.OnReject = node.OnReject

	node.peerConfig = &peer.Config{
		UserAgentName:    config.UserAgentName,
		UserAgentVersion: config.UserAgentVersion,
		ChainParams:      config.Params,
		DisableRelayTx:   false,
		TrickleInterval:  time.Second * 10,
		Listeners:        *listeners,
	}
	return node
}

func (node *Node) OnVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	// Check this peer offers bloom filtering services. If not dump them.
	p.NA().Services = p.Services()
	// Check the full node acceptance. p.NA().HasService(wire.SFNodeNetwork)
	isCash := p.NA().HasService(SFNodeBitcoinCash)
	isSupportBloom := p.NA().HasService(wire.SFNodeBloom)
	isSegwit := p.NA().HasService(wire.SFNodeWitness)

	if isCash {
		log.Infof("Peer %s does not support Bitcoin Cash", p)
		p.Disconnect()
		return
	}
	if !(isSupportBloom && isSegwit) { // Don't connect to bitcoin cash nodes
		// onDisconnection will be called
		// which will remove the peer from openPeers
		log.Infof("Peer %s does not support bloom filtering, diconnecting", p)
		p.Disconnect()
		return
	}
	log.Infof("Connected to %s - %s\n", p.Addr(), p.UserAgent())
}
func (node *Node) OnAddr(p *peer.Peer, msg *wire.MsgAddr) {

}
func (node *Node) OnInv(p *peer.Peer, msg *wire.MsgInv) {
	invVects := msg.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			fmt.Println("inv_block", p.Addr())
			continue
		}
		if invVects[i].Type == wire.InvTypeTx {
			//fmt.Println("inv_tx", invVects[i].Hash, p.Addr())
			iv := invVects[i]
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
			continue
		}
	}
}

func (node *Node) OnTx(p *peer.Peer, msg *wire.MsgTx) {
	tx := msg
	txHash := tx.TxHash()
	fmt.Println("tx", txHash.String(), tx.TxIn, tx.TxOut, p.Addr())
	node.txChan <- Tx{Hash: txHash.String()}
}

func (node *Node) OnReject(p *peer.Peer, msg *wire.MsgReject) {
	log.Warningf("Received reject message from peer %d: Code: %s, Hash %s, Reason: %s", int(p.ID()), msg.Code.String(), msg.Hash.String(), msg.Reason)
}

func (node *Node) ConnectedPeers() []*peer.Peer {
	node.peerMutex.RLock()
	defer node.peerMutex.RUnlock()
	var ret []*peer.Peer
	for _, p := range node.connectedPeers {
		ret = append(ret, p)
	}
	return ret
}

func (node *Node) AddPeer(conn net.Conn) {
	node.peerMutex.Lock()
	defer node.peerMutex.Unlock()
	// Create a new peer for this connection
	p, err := peer.NewOutboundPeer(node.peerConfig, conn.RemoteAddr().String())
	if err != nil {
		p.Disconnect()
		return
	}

	// Associate the connection with the peer
	p.AssociateConnection(conn)

	node.connectedPeers[conn.RemoteAddr().String()] = p

	go func() {
		p.WaitForDisconnect()
		delete(node.connectedPeers, conn.RemoteAddr().String())
		log.Infof("Peer %s disconnected", p)
		// Add new node
		go node.queryDNSSeeds()
	}()
}

// Query the DNS seeds and pass the addresses into the address manager.
func (node *Node) queryDNSSeeds() {
	wg := new(sync.WaitGroup)
	for _, seed := range node.peerConfig.ChainParams.DNSSeeds {
		wg.Add(1)
		go func(host string) {
			var addrs []string
			var err error
			addrs, err = net.LookupHost(host)
			if err != nil {
				wg.Done()
				return
			}
			conns := len(node.ConnectedPeers())
			if uint32(conns) >= node.targetOutbound {
				wg.Done()
				log.Info(conns)
				return
			}
			rn := common.RandRange(0, len(addrs)-1)
			addr := addrs[rn]
			target := &net.TCPAddr{IP: net.ParseIP(addr), Port: 8333}
			conn, err := net.Dial("tcp", target.String())
			if err != nil {
				log.Infof("net.Dial: error %v\n", err)
				return
			}
			node.AddPeer(conn)
			wg.Done()
		}(seed.Host)
	}
	wg.Wait()
}

func (node *Node) Start() {
	go node.queryDNSSeeds()
}

func (node *Node) Stop() {
	node.peerMutex.Lock()
	defer node.peerMutex.Unlock()
	wg := new(sync.WaitGroup)
	for _, peer := range node.connectedPeers {
		wg.Add(1)
		go func() {
			// onDisconnection will be called.
			peer.Disconnect()
			peer.WaitForDisconnect()
			wg.Done()
		}()
	}
	node.connectedPeers = make(map[string]*peer.Peer)
	wg.Wait()
}
