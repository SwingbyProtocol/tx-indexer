# Tx-indexer
The Tx indexer is a memory cache for Bitcoin's pending TX getting via a P2P network. this is suitable to be placed before applications that require real-time processing. The prune function keeps data up to the last few blocks. A subsequent block txs is excluded from the index.

## Usage
```
$ go run ./cmd/tx-indexer --btc.node http://192.168.1.101:8332 --btc.testnet
```
## Configs
```
      --btc.node string      The address for connect btc fullnode (default "http://192.168.1.230:8332")
      --btc.nodeSize int     The maximum node count for connect p2p (default 25)
      --btc.testnet          This is a btc testnet
      --eth.node string      The address for connect eth fullnode (default "http://192.168.1.230:8332")
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
