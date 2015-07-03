#!/bin/bash
mkdir -p results
for INTERVAL in 1s 1m; do
  for WRITE_RATE in 15000 30000 50000; do
    go run main.go -interval=$INTERVAL -writeRate=$WRITE_RATE | tee results/old_metron_${WRITE_RATE}_${INTERVAL}.csv
  done
done
