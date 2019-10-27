#!/bin/bash

docker run -v /var/bitcoind:/bitcoin -d \
    --name=bitcoind-node \
    -p 0.0.0.0:18333:18333 \
    -p 0.0.0.0:18332:18332 \
    kylemanna/bitcoind \
    --prune=812 \
    -rest \
    -rpcbind=0.0.0.0 \
    -rpcallowip=0.0.0.0/0 \
    -minrelaytxfee=0 \
    -maxmempool=300 \
    -mempoolexpiry=72 \
    -rpcworkqueue=100 \
    -testnet=1
