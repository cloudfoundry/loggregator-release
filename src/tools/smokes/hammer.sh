#!/usr/bin/env bash
set -ex

for i in `seq 1 $NUM_APPS`; do
    rm "output-$i.txt" || true
    cf logs drainspinner-$i > output-$i.txt 2>&1 &
done;

sleep 80

echo "Begin the hammer"
for i in `seq 1 $NUM_APPS`; do
    domain=$(cf app drainspinner-$i | grep urls | awk '{print $2}')
    curl "$domain?cycles=${CYCLES}&delay=${DELAY_US}us" &> /dev/null
done;

sleep 25
