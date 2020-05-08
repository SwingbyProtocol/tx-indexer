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
	"github.com/SwingbyProtocol/tx-indexer/config"
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
	log.Infof("AccessToken: %s", conf.AccessToken)

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		ListenREST: conf.RESTConfig.ListenAddr,
		ListenWS:   conf.WSConfig.ListenAddr,
		Actions:    []*api.Action{},
	}

	/* BNB/BEP-2 */
	if conf.BNCConfig.NodeAddr != config.DefaultBNCNode {

		bnbKeeper = bnb.NewKeeper(conf.BNCConfig.NodeAddr, conf.BNCConfig.Testnet)

		bnbKeeper.SetWatchAddr(conf.BNCConfig.WatchAddr)

		go bnbKeeper.Start()

		// BNB side
		getBNBTxs := api.NewGet("/api/v1/bnb/txs", bnbKeeper.GetTxs)
		broadcastBNBTx := api.NewPOST("/api/v1/bnb/broadcast", bnbKeeper.BroadcastTx)

		apiConfig.Actions = append(apiConfig.Actions, getBNBTxs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastBNBTx)
	}

	/* BTC */

	if conf.BTCConfig.NodeAddr != config.DefaultBTCNode {

		btcKeeper = btc.NewKeeper(conf.BTCConfig.NodeAddr, conf.BTCConfig.Testnet, conf.AccessToken)

		err := btcKeeper.SetWatchAddr(conf.BTCConfig.WatchAddr, false, 0)
		if err != nil {
			log.Fatal(err)
		}
		go btcKeeper.Start()

		getBTCTxs := api.NewGet("/api/v1/btc/txs", btcKeeper.GetTxs)
		broadcastBTCTx := api.NewPOST("/api/v1/btc/broadcast", btcKeeper.BroadcastTx)
		setBTCConfig := api.NewPOST("/api/v1/btc/config", btcKeeper.SetConfig)

		apiConfig.Actions = append(apiConfig.Actions, getBTCTxs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastBTCTx)
		apiConfig.Actions = append(apiConfig.Actions, setBTCConfig)
	}

	/* ETH/ERC20 */

	if conf.ETHConfig.NodeAddr != config.DefaultETHNode {

		ethKeeper = eth.NewKeeper(conf.ETHConfig.NodeAddr, conf.ETHConfig.Testnet)

		ethKeeper.SetTokenAndWatchAddr(conf.ETHConfig.WatchToken, conf.ETHConfig.WatchAddr)

		go ethKeeper.Start()

		getERC20Txs := api.NewGet("/api/v1/eth/txs", ethKeeper.GetTxs)
		broadcastETHTx := api.NewPOST("/api/v1/eth/broadcast", ethKeeper.BroadcastTx)

		apiConfig.Actions = append(apiConfig.Actions, getERC20Txs)
		apiConfig.Actions = append(apiConfig.Actions, broadcastETHTx)
	}
	// Create api server
	apiServer := api.NewAPI(apiConfig)

	apiServer.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGSTOP)
	<-c
	if err != nil {
		log.Error(err)
	}
}
