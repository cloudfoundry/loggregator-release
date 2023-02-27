#!/bin/bash

set -eu

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

TMP_DIR=$(mktemp -d)
git clone https://github.com/cloudfoundry/loggregator-api.git $TMP_DIR/loggregator-api

pushd $SCRIPT_DIR/../..
    protoc -I=$TMP_DIR --go_out=. --go-grpc_out=. $TMP_DIR/loggregator-api/v2/*.proto
    mv code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2/* rpc/loggregator_v2/
    rm -rf code.cloudfoundry.org
popd

rm -rf $TMP_DIR
