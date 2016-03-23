#!/bin/bash

set -e

function cleanup {
  pkill -f metron --signal 15 &
  delete_doppler_key
  pkill -f etcd --signal 15 
}

trap "cleanup" EXIT

function delete_doppler_key {
  curl -X DELETE http://localhost:4001/v2/keys/doppler?recursive=true > /dev/null 2>&1
}

function usage {
  cat <<EOF
$0 [options]

$0 runs multiple metronbenchmark iterations, outputting a
summary of each report until the message loss percentage is over
the loss_percent option.  It takes care of spinning up etcd and
metron automatically prior to running the benchmark.  Each
iteration is run for 30 seconds.

options:
  -i, --rate-increment, RATE_INCREMENT
        How much to increase the write rate by each iteration.
  -r, --start-rate, START_RATE
        On the first iteration, how many messages per second
        each writer should write.
  -m, --max-write-rate, MAX_WRITE_RATE
        The upper bound for each writer's write rate.  When this
        number is hit, the iteration will opt to increase the
        number of writers to continue increasing the overall
        write rate.
  -c, --concurrent-writers, STARTING_CONCURRENT_WRITERS
        On the first iteration, how many writers the benchmark
        should start.
  -l, --loss-percent, MAX_MESSAGE_LOSS_PERCENT
        The percentage of message loss to stop the report at.
  -h, --help
        Print this usage message.
EOF
}

function parse_argc {
  RATE_INCREMENT="${RATE_INCREMENT:=1000}"
  START_RATE="${START_RATE:=5000}"
  MAX_WRITE_RATE="${MAX_WRITE_RATE:=10000}"
  STARTING_CONCURRENT_WRITERS="${STARTING_CONCURRENT_WRITERS:=1}"
  MAX_MESSAGE_LOSS_PERCENT="${MAX_MESSAGE_LOSS_PERCENT:=5}"

  while [[ $# -gt 0 ]]
  do
    case "$1" in
      -i|--rate-increment)
        shift
        RATE_INCREMENT=$1
        ;;
      -r|--start-rate)
        shift
        START_RATE=$1
        ;;
      -m|--max-write-rate)
        shift
        MAX_WRITE_RATE=$1
        ;;
      -c|--concurrent-writers)
        shift
        STARTING_CONCURRENT_WRITERS=$1
        ;;
      -l|--loss-percent)
        shift
        MAX_MESSAGE_LOSS_PERCENT=$1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        usage
        exit 1
        ;;
    esac
    shift
  done
}

parse_argc $@

go build github.com/coreos/etcd
go build metron
go build tools/metronbenchmark

./etcd 2>> ./etcd.err.log >> ./etcd.log &

sleep 1 # allow etcd to come up

# sanity check to make sure key got deleted from the last run
delete_doppler_key

loss_percent=0
writers="$STARTING_CONCURRENT_WRITERS"
header_written=false
for (( rate=$START_RATE; $loss_percent < $MAX_MESSAGE_LOSS_PERCENT; rate += $RATE_INCREMENT ))
do
  while [ "$rate" -gt "$MAX_WRITE_RATE" ]
  do
    totalRate="$(($rate * $writers))"
    writers="$(($writers + 1))"
    rate="$(($totalRate / $writers))"
  done;

  ./metron 2>> ./metron.err.log >> ./metron.log &
  result="$(./metronbenchmark -concurrentWriters $writers -writeRate $rate -stopAfter 30s -protocol tls | grep -v "SEND Error")"
  header="$(echo "$result" | head -1)"
  averages="$(echo "$result" | grep "Averages")"
  if ! "$header_written"
  then
    header_written=true
    echo "Averages Value Order: $header"
    echo
  fi

  echo "Results for $writers writers at $rate messages/s each:"
  loss_percent=$(echo $averages | awk ' { split($NF, a, ".") ; print a[1] } ')
  echo "$averages"
done
rate="$(($rate - $RATE_INCREMENT))"

echo "Finished at $writers writers at $rate messages per second, with a $loss_percent% message loss"

