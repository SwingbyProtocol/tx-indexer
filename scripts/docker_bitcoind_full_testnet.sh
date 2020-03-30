#!/bin/bash

docker run -v /var/bitcoind-test:/bitcoin -d \
    --restart=on-failure:10 \
    --name=bitcoind-node-test \
    -p 0.0.0.0:18333:18333 \
    -p 0.0.0.0:18332:18332 \
    kylemanna/bitcoind \
    -rest \
    -rpcbind=0.0.0.0 \
    -rpcallowip=0.0.0.0/0 \
    -minrelaytxfee=0 \
    -maxmempool=300 \
    -mempoolexpiry=10 \
    -rpcworkqueue=200 \
    -txindex=1 \
    -testnet=1
