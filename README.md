# tx-indexer

## Usage
```
go run index.go -bitcoind=http://<bitcoind endpoint>:8332 -prune=12
```
## CMD
```
  -bind string
    	 (default "0.0.0.0:9096")
  -bitcoind string
    	bitcoind endpoint (default "http://localhost:8332")
  -prune int
    	prune blocks (default 4)
  -wsbind string
    	websocket bind (default "0.0.0.0:9099")
```
## WS endpoint
```
ws://localhost:9099/ws
```
### watch new txs of index address
```
{"action":"watchTxs","address":"2N1EY7J5P8YQF2QyUet7RtoDiKvUmAcRs2h"}

response:
{"action":"watchTxs","message":"Success"}
```
### unwatch of index address
```
{"action":"unwatchTxs","address":"2N1EY7J5P8YQF2QyUet7RtoDiKvUmAcRs2h"}
```
### get txs of index address
- params
  - type `string`

```
{"action":"getTxs","address":"mk91p7zsiZrqM57zeBXj2yrh4SHnNsk4Dr","type":"send"}

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