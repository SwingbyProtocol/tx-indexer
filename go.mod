module github.com/SwingbyProtocol/tx-indexer

go 1.13

require (
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/binance-chain/go-sdk v1.2.3
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/ethereum/go-ethereum v1.9.15
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/rs/zerolog v1.19.0
	github.com/shopspring/decimal v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
)

replace github.com/btcsuite/btcd => github.com/SwingbyProtocol/btcd v0.20.1-beta.0.20200305144550-04c526190fa6

replace github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1

replace github.com/zondax/hid => github.com/binance-chain/hid v0.9.1-0.20190807012304-e1ffd6f0a3cc
