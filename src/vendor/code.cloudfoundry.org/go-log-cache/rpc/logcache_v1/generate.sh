#!/bin/bash

set -eu

# Before running you need protoc. Install on ubuntu with `sudo apt install -y protobuf-compiler`

dir_resolve() {
  cd "$1" 2>/dev/null || return $? # cd to desired directory; if fail, quell any error messages but return exit status
  echo "$(pwd -P)"                 # output full, link-resolved path
}

# make it so we can run this script from any directory
TARGET=$(dirname $0)
TARGET=$(dir_resolve $TARGET)
cd $TARGET

# clone dependencies to a temp dir
tmp_dir=$(mktemp -d)
trap "rm -rf $tmp_dir" EXIT

git clone --depth 1 https://github.com/googleapis/googleapis.git $tmp_dir/googleapis
git clone --depth 1 https://github.com/cloudfoundry/loggregator-api $tmp_dir/loggregator-api

OUT_PATH=Mapi/v1/ingress.proto=/logcache_v1,Mapi/v1/egress.proto=/logcache_v1,Mapi/v1/orchestration.proto=/logcache_v1,Mapi/v1/promql.proto=/logcache_v1,Mv2/envelope.proto=code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2:..

protoc \
  api/v1/ingress.proto \
  api/v1/egress.proto \
  api/v1/orchestration.proto \
  api/v1/promql.proto \
  --proto_path=../../ \
  --go_out=$OUT_PATH \
  --go-grpc_out=$OUT_PATH \
  --grpc-gateway_out=$OUT_PATH \
  -I=$tmp_dir/loggregator-api \
  -I=$tmp_dir/googleapis \
