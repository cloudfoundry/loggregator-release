#!/usr/bin/env bash
set -e

for i in `seq 1 $NUM_APPS`; do
    cf delete logspinner-$i -r -f
    rm output-$i.txt || true
done;

count=0
for url in $DRAIN_URLS; do
    cf delete-service ss-smoke-syslog-drain-$count -f
    : $(( count = count + 1 ))
done;

cf delete-service ss-smoke-syslog-https-drain -f
cf delete https-drain -r -f
