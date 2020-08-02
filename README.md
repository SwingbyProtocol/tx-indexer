# Tx-indexer 
[![Go Report Card][1]][2]

[1]: https://goreportcard.com/badge/github.com/SwingbyProtocol/tx-indexer
[2]: https://goreportcard.com/report/github.com/SwingbyProtocol/tx-indexer

Tx-indexer is a modular blockchain transaction monitoring tool. the app can monitor send/receive tx for addresses over a specific period, multiple coins are supported and unified in a common tx format.

## Requirements

This app needs full nodes that fully synced all blocks. there are supported nodes.

- [bitcoin-core](https://github.com/bitcoin/bitcoin) ^v0.18.0 and need to setup rpc endpoint
- [binance-node](https://github.com/binance-chain/node-binary) ^v0.7.2 

## API docs
https://new-testnet-indexer.swingby.network/docs

## Supporting coins
- [x] BTC
- [x] ERC20 (ETH is not support)
- [x] BNB/BEP-2

## Usage
```
$ go run ./cmd/tx-indexer
```
## Flags
```     
      --bnc.node string      The address for connect bnc fullnode (default "tcp://192.168.1.146:26657")
      --bnc.testnet          This is a flag to set testnet-mode on bnc network
      --btc.node string      The address for connect btc fullnode (default "http://bitcoinrpc:DVnnAgN2vYvgS26UR8lH9QJVgbq5nFerycMuwh8P7Sw@192.168.1.146:18332")
      --btc.nodeSize int     The maximum node count for connect p2p fullnode (default 25)
      --btc.testnet          This is a flag to set testnet-mode on btc network
      --eth.node string      The address for connect eth fullnode (default "http://192.168.1.146:8545")
      --eth.testnet          This is a flag to set testnet-mode on eth network
      --eth.token string     The token address for watch (default "0xaff4481d10270f50f203e0763e2597776068cbc5")
      --log.level string     The log level (default "info")
      --prune int            Prune time (hours) (default 24)
  -l, --rest.listen string   The listen address for REST API (default "0.0.0.0:9096")
  -w, --ws.listen string     The listen address for Websocket API (default "0.0.0.0:9099")
```
## Build 
```
$ make build
```
for linux-amd64
```
$ make build-linux-amd64
```

### Build Docker container
```
$ make docker
```
### Run Docker
```
$ export CMD={your command} 
$ docker-compose up -d
```
