# tx-indexer

## Usage
```
go run ./cmd/tx-indexer --node.prune 12
```
## Configs
```
      --node.loglevel string   The loglevel (default "info")
      --node.prune int         Proune block size of this app (default 12)
      --node.testnet           Using testnet
      --p2p.connect string     The address to connect p2p
      --p2p.targetSize int     The maximum node count for connect p2p (default 25)
  -c, --rest.connect string    The address to connect rest (default "http://192.168.1.230:8332")
  -l, --rest.listen string     The listen address for REST API (default "0.0.0.0:9096")
  -w, --ws.listen string       The listen address for Websocket API (default "0.0.0.0:9099")
```

## API reffecrence
### Watch new txs of index address
```
{"action":"watchTxs","address":"1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ"}

response:
{"action":"watchTxs","message":"Success"}
>>>
{"action":"watchTxs","address":"1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ","tx":<*Tx>}
```
### unwatch of index address
```
{"action":"unwatchTxs","address":"1HckjUpRGcrrRAtFaaCAUaGjsPx9oYmLaZ"}

response:
{"action":"unwatchTxs","message":"Success"}
```
### get txs of index address
- params
  - action `string` (*required)
  - address `string` (*required)
  - type `string` only support type text `send`
  - timestamp_from `int64` time window start with linux timestamp
  - timestamp_to   `int64` time window end with linux timestamp

```
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","type":"send","timestamp_from": 1573394833, "timestamp_to":1573394878}

// if you want to get txs which timestamp between 1573394833 and latest
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","type":"send","timestamp_from": 1573394833}

response:
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","txs":<[]*Tx>}
```

### Docker

### bitcoind with prune mode
```
## bitcoind-node with prune mode
mainnet
```
$ chmod +x scripts/docker_bitcoind.sh && scripts/docker_bitcoind.sh
```
testnet
```
$ chmod +x scripts/docker_bitcoind_testnet.sh && scripts/docker_bitcoind_testnet.sh
```