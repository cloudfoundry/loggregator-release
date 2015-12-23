#!/bin/bash

set -e
set -x

FILE=$1

if [ -z $FILE ]; then
  echo "Usage: $0 FILE"
  exit 1
fi

grep msg $FILE | grep -o -E ' [0-9]+\ ' | grep -o -E '[0-9]+' > exp15.lnums
