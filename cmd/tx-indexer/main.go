package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/chains/btc"
	"github.com/SwingbyProtocol/tx-indexer/chains/eth"
	"github.com/SwingbyProtocol/tx-indexer/config"
	log "github.com/sirupsen/logrus"
)

var (
	apiServer = &api.API{}
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
	loglevel := conf.LogConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	btcKeeper := btc.NewKeeper(conf.BTCConfig.NodeAddr, conf.BTCConfig.Testnet)

	btcKeeper.SetAddr("mr6ioeUxNMoavbr2VjaSbPAovzzgDT7Su9")

	btcKeeper.Start()

	uri := os.Getenv("ethRPC")

	keeper := eth.NewKeeper(uri, true)

	token := "0xaff4481d10270f50f203e0763e2597776068cbc5"

	tssAddr := "0x3Ec6671171710F13a1a980bc424672d873b38808"

	keeper.SetTokenAndAddr(token, tssAddr)

	keeper.Start()

	//btcKeeper.StartNode()

	// Define API config
	// Define REST and WS api listener address
	apiConfig := &api.APIConfig{
		ListenREST: conf.RESTConfig.ListenAddr,
		ListenWS:   conf.WSConfig.ListenAddr,
	}

	getBTCTxs := api.NewGet("/api/v1/btc/txs", btcKeeper.GetTxs)
	getERC20Txs := api.NewGet("/api/v1/eth/txs", keeper.GetTxs)

	apiConfig.Actions = []*api.Action{getBTCTxs, getERC20Txs}
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
