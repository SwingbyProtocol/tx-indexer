module github.com/SwingbyProtocol/tx-indexer

go 1.13

require (
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/binance-chain/go-sdk v1.2.2
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/btcsuite/goleveldb v1.0.0 // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/shopspring/decimal v0.0.0-20200419222939-1884f454f8ea
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.3
	google.golang.org/appengine v1.4.0
	google.golang.org/genproto v0.0.0-20200423170343-7949de9c1215 // indirect
)

replace github.com/btcsuite/btcd => github.com/SwingbyProtocol/btcd v0.20.1-beta.0.20200305144550-04c526190fa6

replace github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1

replace github.com/zondax/hid => github.com/binance-chain/hid v0.9.1-0.20190807012304-e1ffd6f0a3cc
