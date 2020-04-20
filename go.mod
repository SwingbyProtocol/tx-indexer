module github.com/SwingbyProtocol/tx-indexer

go 1.13

require (
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/binance-chain/go-sdk v1.2.2
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/btcsuite/btcwallet v0.11.0 // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/rs/zerolog v1.18.0 // indirect
	github.com/shopspring/decimal v0.0.0-20200419222939-1884f454f8ea
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.3
	github.com/ugorji/go v1.1.7 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	google.golang.org/genproto v0.0.0-20200420144010-e5e8543f8aeb // indirect
)

replace github.com/btcsuite/btcd => github.com/SwingbyProtocol/btcd v0.20.1-beta.0.20200305144550-04c526190fa6
