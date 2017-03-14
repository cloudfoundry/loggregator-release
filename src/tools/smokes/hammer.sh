#!/usr/bin/env bash
set -e

for i in `seq 1 5`; do
    rm -f output-$i.txt
    cf logs logspinner-$i > output-$i.txt 2>&1 &
done;

sleep 30 #wait 30 seconds for socket connection

for i in `seq 1 5`; do
    curl "logspinner-$i.$CF_SYSTEM_DOMAIN?cycles=${CYCLES}&delay=${DELAY_US}us" &> /dev/null
done;

sleep 15

msg_count=0
for i in `seq 1 5`; do
    c=$(cat output-$i.txt | grep -c 'msg')
    : $(( msg_count = $msg_count + $c ))
done;

echo $msg_count
