# tx-indexer

## Usage
```
go run index.go -bitcoind=http://<bitcoind endpoint>:8332 -prune=12
```
## CMD
```
  -bitcoind string
    	bitcoind endpoint (default "http://localhost:8332")
  -prune int
    	prune blocks (default 4)
  -rest api bind address string
    	 (default "0.0.0.0:9096")
  -wsbind string
    	websocket bind address (default "0.0.0.0:9099")
```
## WS endpoint
```
ws://localhost:9099/ws
```
## WS requests
### watch new txs of index address
```
{"action":"watchTxs","address":"2N1EY7J5P8YQF2QyUet7RtoDiKvUmAcRs2h"}

response:
{"action":"watchTxs","message":"Success"}
```
### unwatch of index address
```
{"action":"unwatchTxs","address":"2N1EY7J5P8YQF2QyUet7RtoDiKvUmAcRs2h"}

response:
{"action":"unwatchTxs","message":"Success"}
```
### get txs of index address
- params
  - action `string` *required
  - address `string` *required
  - type `string` only support type text `send`
  - timestamp_from `int64` time window start with `linux timestamp`
  - timestamp_to   `int64` time window end with `linux timestamp`

```
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","type":"send","timestamp_from": 1573394833, "timestamp_to":1573394878}

// if you want to get txs which timestamp between 1573394833 and latest
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","type":"send","timestamp_from": 1573394833}

response:
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","txs":<[]*Tx>}
```
## Build
```
$ docker build -t index .
```
## RUN
```
$ docker run -d \
    --restart=always \
    --name index \
    -p 9096:9096 \
    index \
    -prune=12 \
    -bitcoind http://172.17.0.1:8332
```
## bitcoind-node
mainnet
```
$ chmod +x scripts/docker_bitcoind.sh && scripts/docker_bitcoind.sh
```
testnet
```
$ chmod +x scripts/docker_bitcoind_testnet.sh && scripts/docker_bitcoind_testnet.sh
```