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
  -ws string
    	websocket bind (default "0.0.0.0:9099")
```
## WS endpoint
```
ws://localhost:9099/ws
```
- watch/unwatch txs of index address
```
{"action":"watchTxs","address":"1Fi9J5TeaWPHdU5cTJ4e9jr3V58SrWtUuT"}
```
```
{"action":"unwatchTxs","address":"1Fi9J5TeaWPHdU5cTJ4e9jr3V58SrWtUuT"}
```
- get txs of index address
```
{"action":"getTxs","address":"1Fi9J5TeaWPHdU5cTJ4e9jr3V58SrWtUuT"}
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