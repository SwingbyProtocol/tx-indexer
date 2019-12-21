package node

import (
	"fmt"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	SFNodeBitcoinCash wire.ServiceFlag = 1 << 5

	DefaultNodeAddTimes = 4

	DefaultNodeRankSize = uint64(300)
)

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
	TrustedPeer *net.TCPAddr
	// Chan for Tx
	TxChan chan blockchain.Tx
	// Chan for Block
	BlockChan chan blockchain.Block
}

type Node struct {
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	targetOutbound uint32
	connectedRanks map[string]uint64
	connectedPeers map[string]*peer.Peer
	trustedPeer    *net.TCPAddr
	txChan         chan blockchain.Tx
	BlockChan      chan blockchain.Block
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		mu:             new(sync.RWMutex),
		targetOutbound: config.TargetOutbound,
		connectedRanks: make(map[string]uint64),
		connectedPeers: make(map[string]*peer.Peer),
		trustedPeer:    config.TrustedPeer,
		txChan:         config.TxChan,
	}

	listeners := &peer.MessageListeners{}
	listeners.OnVerAck = node.OnVerack
	listeners.OnAddr = node.OnAddr
	listeners.OnInv = node.OnInv
	listeners.OnTx = node.OnTx
	listeners.OnBlock = node.OnBlock
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
	// TODO check addr
}
func (node *Node) OnInv(p *peer.Peer, msg *wire.MsgInv) {
	invVects := msg.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			fmt.Println("inv_block", p.Addr())
			iv := invVects[i]
			//iv.Type = wire.InvTypeFilteredBlock
			gdmsg := wire.NewMsgGetData()
			gdmsg.AddInvVect(iv)
			p.QueueMessage(gdmsg, nil)
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
func (node *Node) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	block := msg
	log.Info(block.BlockHash().String())
	for i, tx := range block.Transactions {
		log.Info(tx.TxHash(), " ", i)
	}
}

func (node *Node) OnTx(p *peer.Peer, msg *wire.MsgTx) {
	tx := msg
	txHash := tx.TxHash()

	go func() {
		node.txChan <- blockchain.Tx{Hash: txHash.String()}
	}()

	// Update node rank
	node.updateRank(p)
	// Get now ranks counts
	top, end, topAddr, olders := node.GetRank()
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
	if len(olders) > 1112 {
		log.Infof("top -> %5d %5d %50s end %50s end2 %50s", top, end, topAddr, olders[0], olders[1])
	}
}

func (node *Node) GetConnectedPeer(addr string) *peer.Peer {
	node.mu.RLock()
	peer := node.connectedPeers[addr]
	node.mu.RUnlock()
	return peer
}

func (node *Node) OnReject(p *peer.Peer, msg *wire.MsgReject) {
	log.Warningf("Received reject message from peer %d: Code: %s, Hash %s, Reason: %s", int(p.ID()), msg.Code.String(), msg.Hash.String(), msg.Reason)
}

func (node *Node) ConnectedPeers() []*peer.Peer {
	node.mu.RLock()
	defer node.mu.RUnlock()
	var ret []*peer.Peer
	for _, p := range node.connectedPeers {
		ret = append(ret, p)
	}
	return ret
}

func (node *Node) GetRank() (uint64, uint64, string, []string) {
	node.mu.RLock()
	top, min, topAddr, olders := common.GetMaxMin(node.connectedRanks)
	node.mu.RUnlock()
	return top, min, topAddr, olders
}

func (node *Node) AddPeer(conn net.Conn) {
	conns := len(node.ConnectedPeers())
	if uint32(conns) >= node.targetOutbound {
		log.Debugf("peer count is enough %d", conns)
		conn.Close()
		return
	}
	log.Info("------- node count ", conns)
	addr := conn.RemoteAddr().String()
	if node.GetConnectedPeer(addr) != nil {
		log.Debugf("peer is already joined %s", addr)
		conn.Close()
		return
	}
	p, err := peer.NewOutboundPeer(node.peerConfig, conn.RemoteAddr().String())
	if err != nil {
		p.Disconnect()
		return
	}
	// Associate the connection with the peer
	p.AssociateConnection(conn)

	// Create a new peer for this connection
	node.mu.Lock()
	node.connectedPeers[conn.RemoteAddr().String()] = p
	node.mu.Unlock()

	go func() {
		p.WaitForDisconnect()
		// Remove peers if peer conn is closed.
		node.mu.Lock()
		delete(node.connectedPeers, conn.RemoteAddr().String())
		node.mu.Unlock()
		log.Debugf("Peer %s disconnected", p)
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
			for i := 0; i < DefaultNodeAddTimes; i++ {
				go func() {
					key := common.RandRange(0, len(addrs)-1)
					addr := addrs[key]
					port, err := strconv.Atoi(node.peerConfig.ChainParams.DefaultPort)
					if err != nil {
						log.Info("port error")
						return
					}
					target := &net.TCPAddr{IP: net.ParseIP(addr), Port: port}
					conn, err := net.Dial("tcp", target.String())
					if err != nil {
						log.Infof("net.Dial: error %v\n", err)
						return
					}
					node.AddPeer(conn)
				}()
			}

			wg.Done()
		}(seed.Host)
	}
	wg.Wait()
}

func (node *Node) updateRank(p *peer.Peer) {
	node.mu.Lock()
	node.connectedRanks[p.Addr()]++
	node.mu.Unlock()
}

func (node *Node) resetConnectedRank() {
	node.connectedRanks = make(map[string]uint64)
}

func (node *Node) Start() {
	if node.trustedPeer != nil {
		conn, err := net.Dial("tcp", node.trustedPeer.String())
		if err != nil {
			log.Infof("net.Dial: error %v\n", err)
			return
		}
		node.AddPeer(conn)
	}
	go node.queryDNSSeeds()
}

func (node *Node) Stop() {
	node.mu.Lock()
	defer node.mu.Unlock()
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
