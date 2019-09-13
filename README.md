# tx-indexer

## Usage
```
go run main.go -bitcoind=http://192.168.1.230:8332 -prune=12 -bind=0.0.0.0:9092
```
## bitcoind-node
mainnet
```
docker run -v /var/bitcoind:/bitcoin  --name=bitcoind-node -d -p 0.0.0.0:8333:8333 -p 0.0.0.0:8332:8332 kylemanna/bitcoind --prune=1812 -rest -rpcbind=0.0.0.0 -rpcallowip=0.0.0.0/0
```
testnet
```
docker run -v /var/bitcoind:/bitcoin  --name=bitcoind-node -d -p 0.0.0.0:18333:18333 -p 0.0.0.0:18332:18332 kylemanna/bitcoind --prune=1812 -testnet=1 -rest -rpcbind=0.0.0.0 -rpcallowip=0.0.0.0/0
```