package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/btc"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
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

var (
	addr = "1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ"
)

func main() {
	txList := getBlockCypher()
	resolver := resolver.NewResolver()
	uri := "http://34.80.154.122:9096"
	query := "/txs/btc/" + addr + "?" + "type=sensd"
	txs := []*btc.Tx{}
	err := resolver.GetRequest(uri, query, &txs)
	if err != nil {
		log.Info(err)
	}
	list := make(map[string]bool)
	for i, tx := range txs {
		if i > 20 {
			continue
		}
		list[tx.Txid] = true
		log.Info(" tx -> ", tx.Txid, " minged -> ", tx.Confirms)
		for _, vout := range tx.Vout {
			if vout.Spent {
				if list[tx.Txid] == true {
					log.Info("     spent --> ", vout.Txs[0], "           ", list[tx.Txid])
				}
			}
		}
	}
	if len(txList) >= 15 {
		txList = txList[:15]
	}
	for _, tx := range txList {
		if list[tx.Hash] == true {
			log.Info("matched -> ", tx.Hash)
		} else {
			log.Info("esle -> ", tx.Hash)
		}
	}
	log.Info(" ---------------> ")
	time.Sleep(10 * time.Second)
	main()
}

type Tx struct {
	Hash     string
	Received string
}

type Base struct {
	Txs []*Tx
}

func getBlockCypher() []Tx {
	resolver := resolver.NewResolver()
	uri := "https://api.blockcypher.com/v1/btc/main/addrs/"
	query := addr + "/full?limit=50?token=a6abe80853fd40da8f9ff05211df8806"
	base := Base{}
	resolver.SetTimeout(12 * time.Second)
	err := resolver.GetRequest(uri, query, &base)
	if err != nil {
		log.Info(err)
	}
	txs := []Tx{}
	for _, tx := range base.Txs {
		txs = append(txs, *tx)
	}

	sort.SliceStable(txs, func(i, j int) bool {
		layout := "2006-01-02T15:04:05"
		ti, err := time.Parse(layout, txs[i].Received[0:19])
		if err != nil {
			fmt.Println(err)
		}
		tj, err := time.Parse(layout, txs[j].Received[0:19])
		if err != nil {
			fmt.Println(err)
		}
		return ti.Unix() > tj.Unix()
	})
	return txs
}
