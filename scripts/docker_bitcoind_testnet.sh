#!/bin/bash

docker run --cpus=0.4 -v /var/bitcoind-test:/bitcoin -d \
    --restart=on-failure:10 \
    --name=bitcoind-node-test \
    -p 0.0.0.0:18333:18333 \
    -p 0.0.0.0:18332:18332 \
    kylemanna/bitcoind \
    --prune=1812 \
    -rest \
    -rpcbind=0.0.0.0 \
    -rpcallowip=0.0.0.0/0 \
    -minrelaytxfee=0 \
    -maxmempool=300 \
    -mempoolexpiry=72 \
    -rpcworkqueue=200 \
    -testnet=1
