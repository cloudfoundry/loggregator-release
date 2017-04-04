#!/usr/bin/env bash
set -ex

cf login -a api.$CF_SYSTEM_DOMAIN -u $CF_USERNAME -p $CF_PASSWORD -s $CF_SPACE -o $CF_ORG --skip-ssl-validation

pushd ./http_drain
    GOOS=linux go build
    cf push https-drain-${DRAIN_VERSION} -c ./http_drain -b binary_buildpack
    drain_domain=$(cf app https-drain-${DRAIN_VERSION} | grep urls | awk '{print $2}')
    cf create-user-provided-service ss-smoke-syslog-https-drain-${DRAIN_VERSION} -l "https://$drain_domain/drain?drain-version=$DRAIN_VERSION" || true
popd

count=0
for i in $DRAIN_URLS; do
    cf create-user-provided-service "ss-smoke-syslog-drain-$count" -l "$url/?drain-version=$DRAIN_VERSION" || true
    : $(( count = count + 1 ))
done;

pushd ../logspinner
    GOOS=linux go build

    for i in `seq 1 $NUM_APPS`; do
        cf push drainspinner-$i -c ./logspinner -b binary_buildpack
        cf bind-service drainspinner-$i ss-smoke-syslog-https-drain-${DRAIN_VERSION}

        count=0
        for url in $DRAIN_URLS; do
            cf bind-service drainspinner-$i ss-smoke-syslog-drain-$count
            : $(( count = count + 1 ))
        done;
    done;
popd
