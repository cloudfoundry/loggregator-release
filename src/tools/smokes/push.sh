#!/usr/bin/env bash
set -e

cf login -a api.$CF_SYSTEM_DOMAIN -u $CF_USERNAME -p $CF_PASSWORD -s $CF_SPACE -o $CF_ORG

pushd ./http_drain
    GOOS=linux go build
    cf push https-drain -c ./http_drain -b binary_buildpack
    cf create-user-provided-service ss-smoke-syslog-https-drain -l "https://https-drain.$CF_SYSTEM_DOMAIN/drain?drain-version=2.0" || true
popd

count=0
for i in $DRAIN_URLS; do
    cf create-user-provided-service "ss-smoke-syslog-drain-$count" -l "$url" || true
    : $(( count = count + 1 ))
done;

pushd ../logspinner
    GOOS=linux go build

    for i in `seq 1 $NUM_APPS`; do
        cf push logspinner-$i -c ./logspinner -b binary_buildpack
        cf bind-service logspinner-$i ss-smoke-syslog-https-drain

        count=0
        for url in $DRAIN_URLS; do
            cf bind-service logspinner-$i ss-smoke-syslog-drain-$count
            : $(( count = count + 1 ))
        done;
    done;
popd

