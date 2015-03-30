#!/bin/bash

set -x 
set -e

cd loggregator

export GOPATH=`pwd`
export TERM=xterm

go install github.com/onsi/ginkgo/ginkgo

bin/test
