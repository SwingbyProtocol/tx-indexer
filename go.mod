module github.com/SwingbyProtocol/tx-indexer

go 1.13

require (
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/binance-chain/go-sdk v1.2.3
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/elastic/gosigar v0.8.1-0.20180330100440-37f05ff46ffa // indirect
	github.com/ethereum/go-ethereum v1.9.15
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/shopspring/decimal v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spaolacci/murmur3 v1.0.1-0.20190317074736-539464a789e9 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	google.golang.org/genproto v0.0.0-20200612171551-7676ae05be11 // indirect
)

replace github.com/btcsuite/btcd => github.com/SwingbyProtocol/btcd v0.20.1-beta.0.20200305144550-04c526190fa6

replace github.com/tendermint/go-amino => github.com/binance-chain/bnc-go-amino v0.14.1-binance.1

replace github.com/zondax/hid => github.com/binance-chain/hid v0.9.1-0.20190807012304-e1ffd6f0a3cc
