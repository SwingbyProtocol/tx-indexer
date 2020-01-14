package node

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

var (
	DefaultNodeAddTimes = 4

	DefaultNodeTimeout = 5 * time.Second

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
	// Chan for Tx
	TxChan chan *wire.MsgTx
	// Chan for Block
	BChan chan *wire.MsgBlock
}

type Node struct {
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	receivedTxs    map[string]bool
	targetOutbound uint32
	connectedRanks map[string]uint64
	connectedPeers map[string]*peer.Peer
	invtxs         map[string]*wire.MsgTx
	txChan         chan *wire.MsgTx
	bChan          chan *wire.MsgBlock
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		mu:             new(sync.RWMutex),
		receivedTxs:    make(map[string]bool),
		invtxs:         make(map[string]*wire.MsgTx),
		connectedRanks: make(map[string]uint64),
		connectedPeers: make(map[string]*peer.Peer),
		targetOutbound: config.TargetOutbound,
		txChan:         config.TxChan,
		bChan:          config.BChan,
	}
	// Override node count times
	if config.Params.Name == "testnet3" {
		DefaultNodeAddTimes = 16
		DefaultNodeRankSize = 100
	}

	listeners := &peer.MessageListeners{}
	listeners.OnVersion = node.onVersion
	listeners.OnVerAck = node.onVerack
	listeners.OnInv = node.onInv
	listeners.OnTx = node.onTx
	listeners.OnBlock = node.onBlock
	listeners.OnReject = node.onReject
	listeners.OnGetData = node.onGetData

	node.peerConfig = &peer.Config{
		UserAgentName:    config.UserAgentName,
		UserAgentVersion: config.UserAgentVersion,
		ChainParams:      config.Params,
		DisableRelayTx:   false,
		TrickleInterval:  time.Second * 10,
		Listeners:        *listeners,
	}
	log.Debugf("Using settings -> DefaultNodeAddTimes: %d DefaultNodeRankSize: %d", DefaultNodeAddTimes, DefaultNodeRankSize)
	return node
}

func (node *Node) Start() {
	go node.queryDNSSeeds()
}

func (node *Node) Stop() {
	node.mu.Lock()
	defer node.mu.Unlock()
	wg := new(sync.WaitGroup)
	peers := node.ConnectedPeers()
	for _, peer := range peers {
		wg.Add(1)
		p := peer
		go func() {
			p.Disconnect()
			p.WaitForDisconnect()
			wg.Done()
		}()
	}
	node.connectedPeers = make(map[string]*peer.Peer)
	wg.Wait()
}

func (node *Node) AddInvTx(txid string, msgTx *wire.MsgTx) {
	node.mu.Lock()
	node.invtxs[txid] = msgTx
	node.mu.Unlock()
}

func (node *Node) GetInvTx(txid string) *wire.MsgTx {
	node.mu.RLock()
	msg := node.invtxs[txid]
	node.mu.RUnlock()
	return msg
}

func (node *Node) RemoveInvTx(txid string) error {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.invtxs[txid] == nil {
		return errors.New("inv is not exist")
	}
	delete(node.invtxs, txid)
	return nil
}

func (node *Node) BroadcastTxInv(txid string) {
	// Get msgTx
	msgTx := node.GetInvTx(txid)
	// Add new tx to invtxs
	var invType wire.InvType
	if msgTx.HasWitness() {
		invType = wire.InvTypeWitnessTx
	} else {
		invType = wire.InvTypeTx
	}
	hash := msgTx.TxHash()
	iv := wire.NewInvVect(invType, &hash)
	// Submit inv msg to network
	node.sendBroadcastInv(iv)
	log.Infof("Broadcast inv msg success: %s", txid)
}

func (node *Node) sendBroadcastInv(iv *wire.InvVect) {
	peers := node.ConnectedPeers()
	if len(peers) > 14 {
		peers = peers[:12]
	}
	for _, peer := range peers {
		peer.QueueInventory(iv)
	}
}

func (node *Node) ConnectedPeer(addr string) *peer.Peer {
	node.mu.RLock()
	peer := node.connectedPeers[addr]
	node.mu.RUnlock()
	return peer
}

func (node *Node) ConnectedPeers() []*peer.Peer {
	node.mu.RLock()
	defer node.mu.RUnlock()
	ret := []*peer.Peer{}
	for _, p := range node.connectedPeers {
		ret = append(ret, p)
	}
	return ret
}

func (node *Node) GetRank() (uint64, uint64, string, []string) {
	node.mu.RLock()
	top, min, topAddr, olders := utils.GetMaxMin(node.connectedRanks)
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
	addr := conn.RemoteAddr().String()
	if node.ConnectedPeer(addr) != nil {
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
		// onDisconnection will be called.
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
	log.Infof("Conncted peers -> %d", len(node.ConnectedPeers()))
	log.Infof("Using network --> %s", node.peerConfig.ChainParams.Name)
}

func (node *Node) addRandomNodes(count int, addrs []string) {
	if count != 0 {
		DefaultNodeAddTimes = count
	}
	for i := 0; i < DefaultNodeAddTimes; i++ {
		key := utils.RandRange(0, len(addrs)-1)
		addr := addrs[key]
		port, err := strconv.Atoi(node.peerConfig.ChainParams.DefaultPort)
		if err != nil {
			log.Debug("port error")
			continue
		}
		target := &net.TCPAddr{IP: net.ParseIP(addr), Port: port}
		dialer := net.Dialer{Timeout: DefaultNodeTimeout}
		conn, err := dialer.Dial("tcp", target.String())
		if err != nil {
			log.Debugf("net.Dial: error %v\n", err)
			continue
		}
		node.AddPeer(conn)
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

func (node *Node) updateRank(p *peer.Peer) {
	node.mu.Lock()
	node.connectedRanks[p.Addr()]++
	node.mu.Unlock()
}

func (node *Node) resetConnectedRank() {
	node.connectedRanks = make(map[string]uint64)
}

func (node *Node) addReceived(hash string) {
	node.mu.Lock()
	node.receivedTxs[hash] = true
	node.mu.Unlock()
	go func() {
		// Delete after 10 min
		time.Sleep(10 * time.Minute)
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
