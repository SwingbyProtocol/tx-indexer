# Tx-indexer
Tx-indexer is a modular blockchain transaction monitoring tool. the app can monitor send/receive tx for a specific address over a specific period, multiple coins are supported and unified in a common tx format.

## Supporting coins
- [x] BTC
- [x] ERC20
- [ ] BNB/BEP-2

## Usage
```
$ go run ./cmd/tx-indexer --btc.node http://192.168.1.101:8332 --btc.testnet
```
## Configs
```
      --btc.node string      The address for connect btc fullnode (default "http://192.168.1.230:8332")
      --btc.nodeSize int     The maximum node count for connect p2p (default 25)
      --btc.testnet          This is a btc testnet
      --eth.node string      The address for connect eth fullnode (default "http://192.168.1.230:8545")
      --eth.testnet          This is a eth testnet
      --log.level string     The log level (default "info")
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
## API reffecrence

- [Websocket client sample code](./examples/websocket_sample/websocket_sample.go)

### Build Docker container
```
$ make docker
```
