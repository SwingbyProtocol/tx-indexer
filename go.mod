module github.com/SwingbyProtocol/tx-indexer

go 1.13

require (
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/binance-chain/go-sdk v1.2.2 // indirect
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/btcsuite/btcwallet v0.11.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/rs/zerolog v1.18.0 // indirect
	github.com/shopspring/decimal v0.0.0-20200227202807-02e2044944cc
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.3
	github.com/tendermint/btcd v0.0.0-20180816174608-e5840949ff4f
	github.com/tendermint/tendermint v0.32.3
	github.com/ugorji/go v1.1.7 // indirect
	google.golang.org/genproto v0.0.0-20200413115906-b5235f65be36 // indirect
)

replace github.com/btcsuite/btcd => github.com/SwingbyProtocol/btcd v0.20.1-beta.0.20200305144550-04c526190fa6
