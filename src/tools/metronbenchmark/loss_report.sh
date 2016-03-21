#!/bin/bash

set -ex

function cleanup {
  pkill -f metron --signal 15 &
  curl -X DELETE http://localhost:4001/v2/keys/doppler?recursive=true
  pkill -f etcd --signal 15 
}

trap "cleanup" EXIT

RATE_INCREASE="${RATE_INCREASE:=1000}"
START_RATE="${START_RATE:=100000}"
LOSS_PERCENT=0

go build github.com/coreos/etcd
go build metron
go build tools/metronbenchmark

./etcd >> ./etcd.log 2>> ./etcd.err.log &

sleep 5 # allow etcd to come up

# sanity check to make sure key got deleted from the last run
curl -X DELETE http://localhost:4001/v2/keys/doppler/meta/z1/doppler_z1/0

for (( rate=$START_RATE; $LOSS_PERCENT < 5; rate += $RATE_INCREASE ))
do
  ./metron >> ./metron.log 2>> ./metron.err.log &
  result="$(./metronbenchmark -writeRate $rate -stopAfter 30s -protocol tls | grep -v "Averages: ")"
  LOSS_PERCENT=$result | awk ' { split($NF, a, ".") ; print a[1] } '
  echo "lost percent: $LOSS_PERCENT"
done
