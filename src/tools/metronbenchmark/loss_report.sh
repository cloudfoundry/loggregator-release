#!/bin/bash

set -e

function cleanup {
  pkill -f metron --signal 15 &
  delete_doppler_key
  pkill -f etcd --signal 15
}

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
  -b, --burst-delay, BURST_DELAY (default: 0)
        With this set, messages will be sent as fast as possible and
        then sleep for the burst delay. If this is set to zero the
        constant rate is used.
        Examples:
          10us = 10 microseconds
          1ms  = 1 millisecond
          3s   = 3 seconds
  -i, --rate-increment, RATE_INCREMENT (default: 1000)
        How much to increase the write rate by each iteration.
  -r, --start-rate, START_RATE (default: 5000)
        On the first iteration, how many messages per second
        (or per burst, if burst-delay is set) each writer
        should write.
  -m, --max-write-rate, MAX_WRITE_RATE (default: 10000)
        The upper bound for each writer's write rate.
  -c, --concurrent-writers, CONCURRENT_WRITERS (default: 1)
        How many writers the benchmark should start.
  -l, --loss-percent, MAX_MESSAGE_LOSS_PERCENT (default: -1)
        The percentage of message loss to stop the report at. If this
        is set, then more concurrent writers will be used after we hit
        max write rate.
  -p, --protocol, PROTOCOL (default: tls)
        The protocol used to communicate between metron and its sink.
        Only this protocol will be advertised by the fake doppler, so
        metron should correctly fall back to this protocol using the
        default config.
  -h, --help
        Print this usage message.
EOF
}

function parse_argc {
  BURST_DELAY="${BURST_DELAY:=0}"
  RATE_INCREMENT="${RATE_INCREMENT:=1000}"
  START_RATE="${START_RATE:=5000}"
  MAX_WRITE_RATE="${MAX_WRITE_RATE:=10000}"
  CONCURRENT_WRITERS="${CONCURRENT_WRITERS:=1}"
  MAX_MESSAGE_LOSS_PERCENT="${MAX_MESSAGE_LOSS_PERCENT:=-1}"
  PROTOCOL="${PROTOCOL:=tls}"

  while [[ $# -gt 0 ]]
  do
    case "$1" in
      -b|--burst-delay)
        shift
        BURST_DELAY=$1
        ;;
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
        CONCURRENT_WRITERS=$1
        ;;
      -l|--loss-percent)
        shift
        MAX_MESSAGE_LOSS_PERCENT=$1
        ;;
      -p|--protocol)
        shift
        PROTOCOL=$1
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

function build_bins {
  go build github.com/coreos/etcd
  go build metron
  go build tools/metronbenchmark
}

function start_etcd {
  ./etcd 2>> ./etcd.err.log >> ./etcd.log &
  sleep 1 # allow etcd to come up
  # sanity check to make sure key got deleted from the last run
  delete_doppler_key
}

function run_benchmark {
  local loss_percent=0
  local header_written=false
  local writers="$CONCURRENT_WRITERS"
  local rate=$START_RATE
  local totalRate="$(($rate * $writers))"

  while true ; do
    # break if we exceeded the max message loss percent and it was set
    if [ "$loss_percent" -gt "$MAX_MESSAGE_LOSS_PERCENT" ] && [ "$MAX_MESSAGE_LOSS_PERCENT" -gt "-1" ] ; then
      break
    fi

    # break if max write rate was hit and no max message loss percent was set
    if [ "$rate" -gt "$MAX_WRITE_RATE" ] && [ "$MAX_MESSAGE_LOSS_PERCENT" -eq "-1" ] ; then
      break
    fi

    # increase concurrent writers
    while [ "$rate" -gt "$MAX_WRITE_RATE" ] ; do
      totalRate="$(($rate * $writers))"
      writers="$(($writers + 1))"
      rate="$(($totalRate / $writers))"
    done

    # spawn metron in the background
    ./metron 2>> ./metron.err.log >> ./metron.log &
    sleep 1

    # spawn metronbenchmark and collect results
    result="$(./metronbenchmark -concurrentWriters $writers -writeRate $rate -stopAfter 30s -protocol $PROTOCOL -burstDelay $BURST_DELAY | grep -v "SEND Error")"
    header="$(echo "$result" | head -1)"
    averages="$(echo "$result" | grep "Averages")"

    # output header
    if ! "$header_written" ; then
      header_written=true
      echo "Averages Value Order: $header"
      echo
    fi

    # output results for this run
    if [ "$BURST_DELAY" = "0" ]; then
      echo "Results for $writers writers at $rate messages/s each:"
    else
      echo "Results for $writers writers at $rate messages every $BURST_DELAY each:"
    fi
    
    echo "$averages"

    # compute loss percentage
    loss_percent=$(echo $averages | awk ' { split($NF, a, ".") ; print a[1] } ')

    # increment rate
    rate="$(( rate + $RATE_INCREMENT ))"
  done

  # need to subtract rate increment here as last iteration will increase it
  final_rate="$(($rate - $RATE_INCREMENT))"

  echo "Finished at $writers writers at $final_rate messages per second, with a $loss_percent% message loss"
}

function main {
  trap "cleanup" EXIT
  parse_argc $@
  build_bins
  start_etcd
  run_benchmark
}
main $@
