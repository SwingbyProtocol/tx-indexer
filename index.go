package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/btc"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

var upgrader = websocket.Upgrader{}

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
	prune := flag.Int("prune", 4, "prune blocks")
	wsBind := flag.String("ws", "0.0.0.0:8080", "websocket bind")
	flag.Parse()

	log.Println("bitcoind ->", *bitcoind, "bind ->", *bind, "prune ->", *prune, "websocket bind ->", *wsBind)

	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	btcNode := btc.NewNode(*bitcoind, *prune)
	btcNode.Start()
	router, err := rest.MakeRouter(
		rest.Get("/keep", func(w rest.ResponseWriter, r *rest.Request) {
			w.WriteHeader(http.StatusOK)
			w.WriteJson([]string{})
		}),
		rest.Get("/txs/btc/:address", btcNode.GetTxs),
		//rest.Get("/txs/btc/tx/:txid", btcNode.GetTx),
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
}
