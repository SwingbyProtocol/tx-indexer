package node

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

var (
	DefualtNodeBootstrapCount = 25

	DefaultNodeAddTimes = 4

	DefaultNodeTimeout = 5 * time.Second
)

type NodeConfig struct {
	IsTestnet bool
	// The target number of outbound peers. Defaults to 10.
	TargetOutbound uint32
	// UserAgentName specifies the user agent name to advertise. It is
	// highly recommended to specify this value.
	UserAgentName string
	// UserAgentVersion specifies the user agent version to advertise.  It
	// is highly recommended to specify this value and that it follows the
	// form "major.minor.revision" e.g. "2.6.41".
	UserAgentVersion string
	// Chan for Tx
	TxChan chan *types.Tx
	// Chan for Block
	BChan chan *types.Block
}

type Node struct {
	start          bool
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	receivedTxs    map[string]bool
	targetOutbound uint32
	ticker         *time.Ticker
	connectedPeers map[string]*peer.Peer
	invtxs         map[string]*wire.MsgTx
	txChan         chan *types.Tx
	bChan          chan *types.Block
}

func NewNode(config *NodeConfig) *Node {
	node := &Node{
		mu:             new(sync.RWMutex),
		receivedTxs:    make(map[string]bool),
		targetOutbound: config.TargetOutbound,
		connectedPeers: make(map[string]*peer.Peer),
		invtxs:         make(map[string]*wire.MsgTx),
		txChan:         config.TxChan,
		bChan:          config.BChan,
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
		DisableRelayTx:   false,
		TrickleInterval:  time.Second * 10,
		Listeners:        *listeners,
	}
	node.peerConfig.ChainParams = &chaincfg.MainNetParams
	if config.IsTestnet {
		node.peerConfig.ChainParams = &chaincfg.TestNet3Params
		DefaultNodeAddTimes = 16
	}
	DefualtNodeBootstrapCount = int(node.targetOutbound)
	log.Infof("Using network -> %s", node.peerConfig.ChainParams.Net.String())
	log.Infof("Using settings -> DefaultNodeAddTimes: %d TargetOutbound %d", DefaultNodeAddTimes, node.targetOutbound)
	return node
}

func (node *Node) Start() {
	node.start = true
	node.ticker = time.NewTicker(12 * time.Second)
	for {
		<-node.ticker.C
		peerCount := len(node.GetConnectedPeers())
		log.Infof("Connected peers: %d", peerCount)
		if peerCount >= DefualtNodeBootstrapCount {
			continue
		}
		node.queryDNSSeeds()
	}
}

func (node *Node) Stop() {
	node.start = false
	node.ticker.Stop()
	peers := node.GetConnectedPeers()
	for _, p := range peers {
		p.Disconnect()
	}
	node.mu.Lock()
	node.connectedPeers = make(map[string]*peer.Peer)
	node.mu.Unlock()
	log.Info("STOP: nodes subscriber stopped")
	return
}

func (node *Node) GetConnectedPeer(addr string) *peer.Peer {
	node.mu.RLock()
	peer := node.connectedPeers[addr]
	node.mu.RUnlock()
	return peer
}

func (node *Node) GetConnectedPeers() []*peer.Peer {
	node.mu.RLock()
	defer node.mu.RUnlock()
	ret := []*peer.Peer{}
	for _, p := range node.connectedPeers {
		ret = append(ret, p)
	}
	return ret
}

// Query the DNS seeds and pass the addresses into the address manager.
func (node *Node) queryDNSSeeds() {
	log.Info("QueryDNSSeeds start...")
	log.Infof("Using network --> %s", node.peerConfig.ChainParams.Name)
	for _, seed := range node.peerConfig.ChainParams.DNSSeeds {
		go func(host string) {
			var addrs []string
			var err error
			addrs, err = net.LookupHost(host)
			if err != nil {
				return
			}
			node.addRandomNodes(addrs)
		}(seed.Host)
	}
}

func (node *Node) addRandomNodes(addrs []string) {
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

func (node *Node) AddPeer(conn net.Conn) {
	conns := len(node.GetConnectedPeers())
	if uint32(conns) >= node.targetOutbound {
		log.Debugf("peer count is enough %d", conns)
		conn.Close()
		return
	}
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
		// onDisconnection will be called.
		node.onDisconneted(p, conn)
	}()
}

func (node *Node) AddInvMsgTx(txid string, msgTx *wire.MsgTx) {
	node.mu.Lock()
	node.invtxs[txid] = msgTx
	node.mu.Unlock()
}

func (node *Node) GetInvMsgTx(txid string) *wire.MsgTx {
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
	msgTx := node.GetInvMsgTx(txid)
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

func (node *Node) pushTxMsg(p *peer.Peer, hash *chainhash.Hash, enc wire.MessageEncoding) error {
	msgTx := node.GetInvMsgTx(hash.String())
	if msgTx == nil {
		return errors.New("Unable to fetch tx from invtxs")
	}
	p.QueueMessageWithEncoding(msgTx, nil, enc)
	log.Infof("Broadcast success txhash: %s peer: %s", hash.String(), p.Addr())
	return nil
}

func (node *Node) sendBroadcastInv(iv *wire.InvVect) {
	peers := node.GetConnectedPeers()
	if len(peers) > 21 {
		peers = peers[:20]
	}
	for _, peer := range peers {
		peer.QueueInventory(iv)
	}
}
