# Tx-indexer
The Tx indexer is a memory cache for Bitcoin's pending TX getting via a P2P network. this is suitable to be placed before applications that require real-time processing. The prune function keeps data up to the last few blocks. A subsequent block txs is excluded from the index.

## Usage
```
$ go run ./cmd/tx-indexer --node.prune 12
```
or 
```
$ make run
```
## Configs
```
      --node.loglevel string   The loglevel (default "info")
      --node.prune int         Proune block size of this app (default 12)
      --node.testnet           Using testnet
      --p2p.connect string     The address to connect p2p
      --p2p.targetSize int     The maximum node count for connect p2p (default 25)
  -c, --rest.connect string    The address for connect rest (default "http://192.168.1.230:8332")
  -l, --rest.listen string     The listen address for REST API (default "0.0.0.0:9096")
  -w, --ws.listen string       The listen address for Websocket API (default "0.0.0.0:9099")
```
## Build 
```
$ make build
```
## API reffecrence

- For REST api
- For Websocket 

### Docker
```
$ make docker
```
### Scripts
```
## bitcoind-node with prune mode
# mainnet
$ chmod +x scripts/docker_bitcoind.sh && scripts/docker_bitcoind.sh  

# testnet
$ chmod +x scripts/docker_bitcoind_testnet.sh && scripts/docker_bitcoind_testnet.sh
```