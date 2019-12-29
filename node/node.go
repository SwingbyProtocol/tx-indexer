package node

import (
	"bytes"
	"encoding/hex"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
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
	// If this field is not nil the PeerManager will only connect to this address
	TrustedPeer string
	// Chan for Tx
	TxChan chan *wire.MsgTx
	// Chan for Block
	BlockChan chan *wire.MsgBlock
}

type Node struct {
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	received       map[string]bool
	targetOutbound uint32
	connectedRanks map[string]uint64
	connectedPeers map[string]*peer.Peer
	invtxs         map[string]*wire.MsgTx
	trustedPeer    string
	txChan         chan *wire.MsgTx
	BlockChan      chan *wire.MsgBlock
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		mu:             new(sync.RWMutex),
		received:       make(map[string]bool),
		targetOutbound: config.TargetOutbound,
		connectedRanks: make(map[string]uint64),
		connectedPeers: make(map[string]*peer.Peer),
		invtxs:         make(map[string]*wire.MsgTx),
		trustedPeer:    config.TrustedPeer,
		txChan:         config.TxChan,
		BlockChan:      config.BlockChan,
	}
	// Override node count times
	if config.Params.Name == "testnet3" {
		DefaultNodeAddTimes = 12
	}

	listeners := &peer.MessageListeners{}
	listeners.OnVersion = node.OnVersion
	listeners.OnVerAck = node.OnVerack
	listeners.OnInv = node.OnInv
	listeners.OnTx = node.OnTx
	listeners.OnBlock = node.OnBlock
	listeners.OnReject = node.OnReject
	listeners.OnGetData = node.OnGetData

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
	peers := node.ConnectedPeers()
	for _, peer := range peers {
		wg.Add(1)
		p := peer
		go func() {
			// onDisconnection will be called.
			p.Disconnect()
			p.WaitForDisconnect()
			wg.Done()
		}()
	}
	node.connectedPeers = make(map[string]*peer.Peer)
	wg.Wait()
}

func (node *Node) DecodeToTx(hexStr string) (*btcutil.Tx, error) {
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, err
	}
	tx := btcutil.NewTx(&msgTx)
	return tx, nil
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

func (node *Node) BroadcastTxInv(txid string) error {
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
	return nil
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
	ret := []*peer.Peer{}
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
	log.Infof("Conncted peers -> %d", len(node.ConnectedPeers()))
}

func (node *Node) addRandomNodes(count int, addrs []string) {
	if count != 0 {
		DefaultNodeAddTimes = count
	}
	for i := 0; i < DefaultNodeAddTimes; i++ {
		key := common.RandRange(0, len(addrs)-1)
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

func (node *Node) updateRank(p *peer.Peer) {
	node.mu.Lock()
	node.connectedRanks[p.Addr()]++
	node.mu.Unlock()
}

func (node *Node) resetConnectedRank() {
	node.connectedRanks = make(map[string]uint64)
}
