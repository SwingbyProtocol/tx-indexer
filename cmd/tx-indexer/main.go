package main

import (
	"net"
	"os"

	"github.com/SwingbyProtocol/tx-indexer/common/config"
	"github.com/SwingbyProtocol/tx-indexer/node"
	"github.com/btcsuite/btcd/chaincfg"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func handlerGetTxs() {

}

func main() {
	_, err := config.NewDefaultConfig()
	if err != nil {
		log.Info(err)
	}
	nodeConfig := &node.NodeConfig{
		Params:           &chaincfg.MainNetParams,
		TargetOutbound:   100,
		UserAgentName:    "test",
		UserAgentVersion: "0.1.0",
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

	log.Info("Default REST Node", config.Set.RESTConfig.ConnAddr)

	node := node.NewNode(nodeConfig)

	node.Start()

	//apiConfig := api.Config{}

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
