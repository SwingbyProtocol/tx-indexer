# tx-indexer

## Usage
```
go run index.go -bitcoind=http://<bitcoind endpoint>:8332 -prune=12
```
## Docker run
```
$ docker build -t index . && docker run -d -v \
    --name=index \
    -p 9096:9096 \
    index \
    -prune=12 \
    -bitcoind http://<bitcoind endpoint>:8332 
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