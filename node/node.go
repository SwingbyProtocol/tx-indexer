package node

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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
	TxChan chan *types.Tx
	// Chan for Block
	BlockChan chan *wire.MsgBlock
}

type Node struct {
	peerConfig     *peer.Config
	mu             *sync.RWMutex
	receivedTxs    map[string]bool
	restNodes      map[string]bool
	targetOutbound uint32
	connectedRanks map[string]uint64
	connectedPeers map[string]*peer.Peer
	invtxs         map[string]*wire.MsgTx
	trustedPeer    string
	txChan         chan *types.Tx
	blockChan      chan *wire.MsgBlock
}

func NewNode(config *NodeConfig) *Node {

	node := &Node{
		mu:             new(sync.RWMutex),
		receivedTxs:    make(map[string]bool),
		restNodes:      make(map[string]bool),
		targetOutbound: config.TargetOutbound,
		connectedRanks: make(map[string]uint64),
		connectedPeers: make(map[string]*peer.Peer),
		invtxs:         make(map[string]*wire.MsgTx),
		trustedPeer:    config.TrustedPeer,
		txChan:         config.TxChan,
		blockChan:      config.BlockChan,
	}
	// Override node count times
	if config.Params.Name == "testnet3" {
		DefaultNodeAddTimes = 16
		DefaultNodeRankSize = 100
	}
	// Add bootstrap nodes
	node.restNodes["35.185.181.99:18332"] = true
	node.restNodes["18.216.48.162:18332"] = true

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
	log.Infof("Using network -> %s", config.Params.Name)
	log.Infof("Using settings -> DefaultNodeAddTimes: %d DefaultNodeRankSize: %d", DefaultNodeAddTimes, DefaultNodeRankSize)
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

func (node *Node) ValidateTx(tx *btcutil.Tx) error {
	err := utils.CheckNonStandardTx(tx)
	if err != nil {
		return err
	}
	err = utils.OutputRejector(tx.MsgTx())
	if err != nil {
		return err
	}
	return nil
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

func (node *Node) GetNodes() []string {
	node.mu.RLock()
	defer node.mu.RUnlock()
	list := []string{}
	for addr, active := range node.restNodes {
		if active == true {
			list = append(list, addr)
		}
	}
	return list
}

func (node *Node) ScanRestNodes() {
	wg := new(sync.WaitGroup)
	node.mu.RLock()
	nodes := node.restNodes
	node.mu.RUnlock()
	for addr := range nodes {
		wg.Add(1)
		go func(addr string) {
			resolver := api.NewResolver("http://" + addr)
			resolver.SetTimeout(3 * time.Second)
			info := types.ChainInfo{}
			err := resolver.GetRequest("/rest/chaininfo.json", &info)
			if err != nil {
				node.mu.Lock()
				delete(node.restNodes, addr)
				node.mu.Unlock()
				wg.Done()
				return
			}
			node.mu.Lock()
			node.restNodes[addr] = true
			node.mu.Unlock()
			wg.Done()
		}(addr)
	}
	wg.Wait()
	count := len(nodes)
	log.Infof("rest nodes -> %d", count)
	if count < 3 {
		go func() {
			log.Info("start queryDNSSeeds")
			time.Sleep(5 * time.Second)
			node.queryDNSSeeds()
		}()
	}
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
			node.addRandomNodes(addrs)
			wg.Done()
		}(seed.Host)
	}
	wg.Wait()
	log.Infof("Conncted peers -> %d", len(node.ConnectedPeers()))
	log.Infof("Using network --> %s", node.peerConfig.ChainParams.Name)
	// rescan all of restnodes
	node.ScanRestNodes()
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

		go func() {
			httpTarget := &net.TCPAddr{IP: net.ParseIP(addr), Port: 18332}
			node.mu.Lock()
			node.restNodes[httpTarget.String()] = false
			node.mu.Unlock()
		}()

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

func (node *Node) sendBroadcastInv(iv *wire.InvVect) {
	peers := node.ConnectedPeers()
	if len(peers) > 21 {
		peers = peers[:20]
	}
	for _, peer := range peers {
		peer.QueueInventory(iv)
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
