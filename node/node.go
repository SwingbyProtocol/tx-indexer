package node

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

var (
	SFNodeBitcoinCash wire.ServiceFlag = 1 << 5

	DefaultNodeAddTimes = 4

	DefaultNodeRankSize = uint64(300)
)

type Node struct {
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	received       map[string]bool
	targetOutbound uint32
	connectedRanks map[string]uint64
	connectedPeers map[string]*peer.Peer
	trustedPeer    string
	txChan         chan blockchain.Tx
	BlockChan      chan blockchain.Block
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		mu:             new(sync.RWMutex),
		received:       make(map[string]bool),
		targetOutbound: config.TargetOutbound,
		connectedRanks: make(map[string]uint64),
		connectedPeers: make(map[string]*peer.Peer),
		trustedPeer:    config.TrustedPeer,
		txChan:         config.TxChan,
		BlockChan:      config.BlockChan,
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

func (node *Node) Start() {
	if node.trustedPeer != "" {
		conn, err := net.Dial("tcp", node.trustedPeer)
		if err != nil {
			log.Fatal("net.Dial: error %v\n", err)
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

func (node *Node) OnVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	// Check this peer offers bloom filtering services. If not dump them.
	p.NA().Services = p.Services()
	// Check the full node acceptance. p.NA().HasService(wire.SFNodeNetwork)
	isCash := p.NA().HasService(SFNodeBitcoinCash)
	isSupportBloom := p.NA().HasService(wire.SFNodeBloom)
	isSegwit := p.NA().HasService(wire.SFNodeWitness)

	if isCash {
		log.Debugf("Peer %s does not support Bitcoin Cash", p)
		p.Disconnect()
		return
	}
	if !(isSupportBloom && isSegwit) { // Don't connect to bitcoin cash nodes
		// onDisconnection will be called
		// which will remove the peer from openPeers
		log.Debugf("Peer %s does not support bloom filtering, diconnecting", p)
		p.Disconnect()
		return
	}
	log.Debugf("Connected to %s - %s - [segwit: %t]", p.Addr(), p.UserAgent(), isSegwit)
}

func (node *Node) OnAddr(p *peer.Peer, msg *wire.MsgAddr) {
	// TODO check addr
}
func (node *Node) OnReject(p *peer.Peer, msg *wire.MsgReject) {
	log.Warningf("Received reject message from peer %d: Code: %s, Hash %s, Reason: %s", int(p.ID()), msg.Code.String(), msg.Hash.String(), msg.Reason)
}
func (node *Node) OnInv(p *peer.Peer, msg *wire.MsgInv) {
	invVects := msg.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			fmt.Println("inv_block", p.Addr())
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
func (node *Node) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	msgBlock := msg
	block := blockchain.Block{
		Hash: msgBlock.BlockHash().String(),
	}
	for _, msgTx := range msgBlock.Transactions {
		tx := blockchain.MsgTxToTx(msgTx)
		block.Txs = append(block.Txs, &tx)
	}
	go func() {
		node.BlockChan <- block
	}()

	log.Infof("block %s %s", block.Hash, p.Addr())
}

func (node *Node) OnTx(p *peer.Peer, msg *wire.MsgTx) {
	msgTx := msg
	//isWitness := msgTx.HasWitness()
	// Update node rank
	node.updateRank(p)
	// Get now ranks counts
	top, min, _, olders := node.GetRank()
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

	txHash := msgTx.TxHash().String()
	if node.isTxReceived(txHash) {
		return
	}
	node.addTxReceived(txHash)

	tx := blockchain.MsgTxToTx(msgTx)

	go func() {
		node.txChan <- tx
	}()

	if len(olders) > 2 {
		log.Debugf("%s  %s  top %d min %d rm %s and %s", tx.Txid, tx.WitnessID, top, min, olders[0], olders[1])
	}
}

func (node *Node) addTxReceived(txHash string) {
	node.mu.Lock()
	node.received[txHash] = true
	node.mu.Unlock()
}

func (node *Node) isTxReceived(txHash string) bool {
	node.mu.RLock()
	bool := node.received[txHash]
	node.mu.RUnlock()
	return bool
}

func (node *Node) GetConnectedPeer(addr string) *peer.Peer {
	node.mu.RLock()
	peer := node.connectedPeers[addr]
	node.mu.RUnlock()
	return peer
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
	log.Infof("------- node count %d %s", conns, conn.RemoteAddr().String())
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
		node.onDisconneted(p, conn)
	}()
}

func (node *Node) onDisconneted(p *peer.Peer, conn net.Conn) {
	// Remove peers if peer conn is closed.
	node.mu.Lock()
	delete(node.connectedPeers, conn.RemoteAddr().String())
	node.mu.Unlock()
	log.Infof("Peer %s disconnected", p)
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
			node.addRandomNodes(0, addrs)
			wg.Done()
		}(seed.Host)
	}
	wg.Wait()
}

func (node *Node) addRandomNodes(count int, addrs []string) {
	if count != 0 {
		DefaultNodeAddTimes = count
	}
	for i := 0; i < DefaultNodeAddTimes; i++ {
		go func() {
			key := common.RandRange(0, len(addrs)-1)
			addr := addrs[key]
			port, err := strconv.Atoi(node.peerConfig.ChainParams.DefaultPort)
			if err != nil {
				log.Debug("port error")
				return
			}
			target := &net.TCPAddr{IP: net.ParseIP(addr), Port: port}
			conn, err := net.Dial("tcp", target.String())
			if err != nil {
				log.Debugf("net.Dial: error %v\n", err)
				return
			}
			node.AddPeer(conn)
		}()
	}
}

func (node *Node) updateRank(p *peer.Peer) {
	node.mu.Lock()
	node.connectedRanks[p.Addr()]++
	node.mu.Unlock()
}

func (node *Node) resetConnectedRank() {
	node.connectedRanks = make(map[string]uint64)
}
