#!/usr/bin/env bash
set -e

for i in `seq 1 5`; do
    cf delete logspinner-$i -r -f
    rm output-$i.txt
done;

cf delete-service ss-smoke-syslog-drain -f
