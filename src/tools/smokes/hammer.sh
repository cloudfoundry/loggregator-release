#!/usr/bin/env bash
set -e

for i in `seq 1 $NUM_APPS`; do
    rm -f output-$i.txt
    cf logs logspinner-$i > output-$i.txt 2>&1 &
done;

sleep 30 #wait 30 seconds for socket connection

echo "Begin the hammer"
for i in `seq 1 $NUM_APPS`; do
    curl "logspinner-$i.$CF_SYSTEM_DOMAIN?cycles=${CYCLES}&delay=${DELAY_US}us" &> /dev/null
done;

sleep 25
