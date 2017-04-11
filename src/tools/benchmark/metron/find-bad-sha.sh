#!/usr/bin/env bash

go build metron_benchmark.go

for SHA in `git rev-list v78..HEAD`; do
    echo $SHA
    git co $SHA
    go build metron
    ./metron_benchmark \
      -cmd "$PWD/metron --config metron_config.json" \
      -tag $SHA \
      -ca ca.cert \
      -cert doppler.cert \
      -key doppler.key \
      -doppler localhost:9999 \
      -metron localhost:10002 \
      -iter 100000 \
      -cycles 3 \
      2> /dev/null > /tmp/results-metron
done

