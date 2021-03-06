package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/chains/bnb"
	"github.com/SwingbyProtocol/tx-indexer/chains/btc"
	"github.com/SwingbyProtocol/tx-indexer/chains/eth"
	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/config"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

var (
	bnbKeeper *bnb.Keeper
	btcKeeper *btc.Keeper
	ethKeeper *eth.Keeper
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
			paddedFuncname := fmt.Sprintf(" %-30v", funcname+"()")
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
	loglevel := conf.LogConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	log.Infof("Set prune duration: %d hours", conf.PruneTime)

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		ListenREST: conf.RESTConfig.ListenAddr,
		ListenWS:   conf.WSConfig.ListenAddr,
		Actions:    []*api.Action{},
	}

	status := func(w rest.ResponseWriter, r *rest.Request) {
		w.WriteJson(common.Response{Result: true})
	}
	getStatus := api.NewGet("/api/v1/status", status)
	apiConfig.Actions = append(apiConfig.Actions, getStatus)

	/* BNB/BEP-2 */
	if conf.BNCConfig.NodeAddr != config.DefaultBNCNode {

		bnbKeeper = bnb.NewKeeper(conf.BNCConfig.NodeAddr, conf.BNCConfig.Testnet, ".data/bnb", conf.PruneTime)
		// BNB side
		getBNBTxs := api.NewGet("/api/v1/bnb/txs", bnbKeeper.GetTxs)
		getBNBTx := api.NewGet("/api/v1/bnb/tx", bnbKeeper.GetTx)
		//getMemoTxs := api.NewGet("/api/v1/bnb/memo_txs", bnbKeeper.GetMemoTxs)
		//getSelfSendTxs := api.NewGet("/api/v1/bnb/self", bnbKeeper.GetSelfSendTxs)
		broadcastBNBTx := api.NewPOST("/api/v1/bnb/broadcast", bnbKeeper.BroadcastTx)

		apiConfig.Actions = append(apiConfig.Actions, getBNBTxs)
		apiConfig.Actions = append(apiConfig.Actions, getBNBTx)
		//apiConfig.Actions = append(apiConfig.Actions, getMemoTxs)
		//apiConfig.Actions = append(apiConfig.Actions, getSelfSendTxs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastBNBTx)
	}

	/* ETH/ERC20 */

	if conf.ETHConfig.NodeAddr != config.DefaultETHNode {

		ethKeeper = eth.NewKeeper(conf.ETHConfig.NodeAddr, conf.ETHConfig.Testnet)

		ethKeeper.SetToken(conf.ETHConfig.WatchToken, "Sample Test Token", 18)

		getERC20Txs := api.NewGet("/api/v1/eth/txs", ethKeeper.GetTxs)
		broadcastETHTx := api.NewPOST("/api/v1/eth/broadcast", ethKeeper.BroadcastTx)

		apiConfig.Actions = append(apiConfig.Actions, getERC20Txs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastETHTx)
	}

	/* BTC */

	if conf.BTCConfig.NodeAddr != config.DefaultBTCNode {

		btcKeeper = btc.NewKeeper(conf.BTCConfig.NodeAddr, conf.BTCConfig.Testnet, ".data/btc", conf.PruneTime)

		getBTCTxs := api.NewGet("/api/v1/btc/txs", btcKeeper.GetTxs)
		getBTCTx := api.NewGet("/api/v1/btc/tx", btcKeeper.GetTx)
		broadcastBTCTx := api.NewPOST("/api/v1/btc/broadcast", btcKeeper.BroadcastTx)

		apiConfig.Actions = append(apiConfig.Actions, getBTCTx)
		apiConfig.Actions = append(apiConfig.Actions, getBTCTxs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastBTCTx)
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)
	// Start server
	apiServer.Start()
	// Start bnbKeeper
	if bnbKeeper != nil {
		bnbKeeper.Start()
	}
	// Start ethKeeper
	if ethKeeper != nil {
		ethKeeper.Start()
	}
	// Start btcKeeper
	if btcKeeper != nil {
		btcKeeper.Start()
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGSTOP)
	<-c
	if err != nil {
		log.Error(err)
	}
}
