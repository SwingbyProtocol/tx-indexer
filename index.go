package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/SwingbyProtocol/sc-indexer/btc"
	"github.com/ant0ine/go-json-rest/rest"
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

func main() {
	bitcoind := flag.String("bitcoind", "http://localhost:8332", "bitcoind endpoint")
	bind := flag.String("bind", "0.0.0.0:9096", "")
	prune := flag.Int64("prune", 4, "prune blocks")
	flag.Parse()
	log.Println("bitcoind ->", *bitcoind, "bind ->", *bind, "prune ->", *prune)
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	btcNode := btc.NewNode(*bitcoind, *prune)
	btcNode.Start()
	router, err := rest.MakeRouter(
		rest.Get("/txs/btc/:address", btcNode.GetBTCTxs),
		rest.Get("/txs/btc/tx/:txid", btcNode.GetBTCTx),
		rest.Get("/txs/btc/index/:address", btcNode.GetBTCIndex),
	)
	if err != nil {
		log.Fatal(err)
	}
	api.SetApp(router)
	log.Fatal(http.ListenAndServe(*bind, api.MakeHandler()))
}
