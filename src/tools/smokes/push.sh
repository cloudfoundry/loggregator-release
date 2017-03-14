#!/usr/bin/env bash
set -e

cf login -a api.$CF_SYSTEM_DOMAIN -u $CF_USERNAME -p $CF_PASSWORD -s $CF_SPACE -o $CF_ORG

cf create-user-provided-service ss-smoke-syslog-drain -l $DRAIN_URL || true

for i in `seq 1 5`; do
    cf push logspinner-$i -p ../logspinner
    cf bind-service logspinner-$i ss-smoke-syslog-drain
done;
