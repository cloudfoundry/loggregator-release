#!/usr/bin/env bash

# go build doppler_benchmark.go

# for SHA in `git rev-list v78..develop`; do
    go build doppler
    ./doppler_benchmark \
      -cmd "$PWD/doppler --config doppler_config.json" \
      -ca ca.cert \
      -cert doppler.cert \
      -key doppler.key \
      -doppler 127.0.0.1:9999 \
      -iter 1000000 \
      -delay 0ns \
      -cycles 3 #\
      # 2>> /dev/null >> /tmp/results-doppler
# done

