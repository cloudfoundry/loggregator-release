#!/bin/bash

set -x 
set -e

env

mkdir ${PWD}/gotemp

go version

export GOPATH=${PWD}/gotemp

go get golang.org/x/tools/cmd/vet
go get golang.org/x/tools/cmd/cover

export GOROOT=/usr/src/go
export PATH=${GOPATH}/bin:${GOROOT}/bin:/bin:/usr/bin:/usr/local/bin
export TERM=xterm

env

go install github.com/onsi/ginkgo/ginkgo

cd $PWD/loggregator

bin/test
