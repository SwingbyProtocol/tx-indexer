package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/SwingbyProtocol/tx-indexer/proxy"
	"github.com/SwingbyProtocol/tx-indexer/types"
	log "github.com/sirupsen/logrus"
)

var (
	bc        = &blockchain.Blockchain{}
	apiServer = &api.API{}
	peer      = &node.Node{}
)

func init() {
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcname := s[len(s)-1]
			//_, filename := path.Split(f.File)
			paddedFuncname := fmt.Sprintf(" %-20v", funcname+"()")
			//paddedFilename := fmt.Sprintf("%17v", filename)
			return paddedFuncname, ""
		},
	})
	log.SetOutput(os.Stdout)
}

func main() {
	conf, err := config.NewDefaultConfig()
	if err != nil {
		log.Fatal(err)
	}
	loglevel := conf.NodeConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	// Create rest peers
	proxy := proxy.NewProxy()
	// Create bc config
	bcConfig := &blockchain.BlockchainConfig{
		TxChan:      make(chan *types.Tx, 100000),
		BChan:       make(chan *types.Block),
		PushMsgChan: make(chan *types.PushMsg),
		Proxy:       proxy,
	}
	// Create blockchain instance
	bc = blockchain.NewBlockchain(bcConfig)
	// Start blockchain service
	bc.Start()

	nodeConfig := &node.NodeConfig{
		Params:           conf.P2PConfig.Params,
		TargetOutbound:   conf.P2PConfig.TargetOutbound,
		UserAgentName:    "Tx-indexer",
		UserAgentVersion: "1.0.0",
		TxChan:           bcConfig.TxChan,
		BChan:            bcConfig.BChan,
	}
	log.Infof("Using network -> %s", nodeConfig.Params.Name)
	// Peer initialize
	peer = node.NewNode(nodeConfig)
	// Peer Start
	peer.Start()

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		ListenREST: conf.RESTConfig.ListenAddr,
		ListenWS:   conf.WSConfig.ListenAddr,
		Actions:    []*api.Action{},
	}
	// Add new handler for watchTxs action
	watchTxs := api.NewWatch("watchTxs", onWatchTxWS)
	apiConfig.AddAction(watchTxs)
	// Add new handler for unwatchTxs action
	unwatchTxs := api.NewWatch("unwatchTxs", onUnWatchTxWS)
	apiConfig.AddAction(unwatchTxs)
	// Add new handler for unwatchTxs action
	getTxs := api.NewWatch("getTxs", onGetIndexTxsWS)
	apiConfig.AddAction(getTxs)
	// Add new handler for unwatchTxs action
	broadcast := api.NewWatch("broadcast", onBroadcastTxWS)
	apiConfig.AddAction(broadcast)

	// Create api server
	apiServer := api.NewAPI(apiConfig)

	// Add handler for REST
	//apiConfig.Listeners.OnGetTx = bc.OnGetTx
	//apiConfig.Listeners.OnGetAddressIndex = bc.OnGetAddressIndex

	apiServer.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGSTOP)
	signal := <-c
	if err != nil {
		log.Error(err)
	}
	log.Info(signal)
}
