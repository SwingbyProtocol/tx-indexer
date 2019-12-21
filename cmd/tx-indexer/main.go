package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/blockchain"
	"github.com/SwingbyProtocol/tx-indexer/common/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			s := strings.Split(f.Function, ".")
			funcname := s[len(s)-1]
			_, filename := path.Split(f.File)
			padded := fmt.Sprintf("%-12v", funcname+"()")
			return padded, filename
		},
	})
}

func main() {
	_, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	loglevel := config.Set.NodeConfig.LogLevel
	if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	// Create blockchain instance
	blockchain := blockchain.NewBlockchain()
	// Start blockchain service
	blockchain.Start()

	nodeConfig := &node.NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   100,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
		TxChan:           blockchain.TxChan(),
		BlockChan:        blockchain.BlockChan(),
	}
	// Add trusted P2P Node
	addr := config.Set.P2PConfig.ConnAddr
	if addr != "" {
		trustedPeer, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}
		nodeConfig.TrustedPeer = trustedPeer
		log.Info("Default P2P Node", addr)
	}
	// Node initialize
	node := node.NewNode(nodeConfig)
	// Node Start
	node.Start()

	// Define REST api listener address
	restListener, err := net.ResolveTCPAddr("tcp", config.Set.RESTConfig.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	wsListener, err := net.ResolveTCPAddr("tcp", config.Set.WSConfig.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	// Define API config
	apiConfig := api.Config{
		RESTListen: restListener,
		WSListen:   wsListener,
	}

	log.Info(apiConfig)

	select {}

	/*
		bitcoind := flag.String("bitcoind", "http://localhost:8332", "bitcoind endpoint")
		bind := flag.String("restbind", "0.0.0.0:9096", "rest api bind address")
		prune := flag.Int("prune", 4, "prune blocks")
		wsBind := flag.String("wsbind", "0.0.0.0:9099", "websocket bind address")
		flag.Parse()

		log.Println("bitcoind ->", *bitcoind, "rest api bind ->", *bind, "prune ->", *prune, "websocket api bind ->", *wsBind+"/ws")

		api := rest.NewApi()
		api.Use(rest.DefaultDevStack...)
		btcNode := btc.NewPeer(*bitcoind, *prune)
		btcNode.Start()
		router, err := rest.MakeRouter(
			rest.Get("/keep", func(w rest.ResponseWriter, r *rest.Request) {
				w.WriteHeader(http.StatusOK)
				w.WriteJson([]string{})
			}),
			rest.Get("/txs/btc/:address", btcNode.GetTxs),
			rest.Get("/txs/btc/tx/:txid", btcNode.GetTx),
			//rest.Get("/txs/btc/index/:address", btcNode.GetIndex),
		)
		if err != nil {
			log.Fatal(err)
		}
		api.SetApp(router)
		go func() {
			err = http.ListenAndServe(*bind, api.MakeHandler())
			if err != nil {
				log.Fatal(err)
			}
		}()

		// ws
		http.HandleFunc("/ws", btcNode.WsHandler)
		err = http.ListenAndServe(*wsBind, nil)
		if err != nil {
			log.Fatal(err)
		}
	*/
}
