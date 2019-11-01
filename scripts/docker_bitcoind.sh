#!/bin/bash

docker run -v /var/bitcoind:/bitcoin -d \
    --restart=always \
    --name=bitcoind-node \
    -p 0.0.0.0:8333:8333 \
    -p 0.0.0.0:8332:8332 \
    kylemanna/bitcoind \
    --prune=812 \
    -rest \
    -rpcbind=0.0.0.0 \
    -rpcallowip=0.0.0.0/0 \
    -minrelaytxfee=0 \
    -maxmempool=300 \
    -mempoolexpiry=72 \
    -rpcworkqueue=100
