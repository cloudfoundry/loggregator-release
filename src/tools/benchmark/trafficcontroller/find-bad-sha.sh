#!/usr/bin/env bash

go build trafficcontroller_benchmark.go

# etcd &
# etcd_pid=$!
# function kill_etcd {
#     kill -9 $etcd_pid
# }
# trap EXIT kill_etcd

for SHA in `git rev-list develop~1..develop`; do
    echo $SHA
    git co $SHA
    go build trafficcontroller
    ./trafficcontroller_benchmark \
      -cmd "$PWD/trafficcontroller --config trafficcontroller_config.json --disableAccessControl" \
      -tag $SHA \
      -ca ca.cert \
      -cert doppler.cert \
      -key doppler.key \
      -doppler 127.0.0.1:9999 \
      -trafficcontroller ws://127.0.0.1:10000 \
      -iter 1000000 \
      -delay 0ns \
      -etcd 127.0.0.1:2379 \
      -cycles 3 #\
      #2>> /dev/null >> /tmp/results-doppler
done

