#!/usr/bin/env bash
set -e

[ $# -ge 1 -a -f "$1" ] && input="$1" || input="-"

msg_count=$(cat $input)
currenttime=$(date +%s)

curl  -X POST -H "Content-type: application/json" \
-d "{ \"series\" :
       [{\"metric\":\"smoke_test.ss.loggregator.msg_count\",
        \"points\":[[${currenttime}, ${msg_count}]],
        \"type\":\"gauge\",
        \"host\":\"${CF_SYSTEM_DOMAIN}\",
        \"tags\":[]}
      ]
  }" \
"https://app.datadoghq.com/api/v1/series?api_key=$DATADOG_API_KEY"

curl  -X POST -H "Content-type: application/json" \
-d "{ \"series\" :
       [{\"metric\":\"smoke_test.ss.loggregator.delay\",
        \"points\":[[${currenttime}, ${DELAY_US}]],
        \"type\":\"gauge\",
        \"host\":\"${CF_SYSTEM_DOMAIN}\",
        \"tags\":[]}
      ]
  }" \
"https://app.datadoghq.com/api/v1/series?api_key=$DATADOG_API_KEY"

curl  -X POST -H "Content-type: application/json" \
-d "{ \"series\" :
       [{\"metric\":\"smoke_test.ss.loggregator.cycles\",
        \"points\":[[${currenttime}, ${CYCLES}]],
        \"type\":\"gauge\",
        \"host\":\"${CF_SYSTEM_DOMAIN}\",
        \"tags\":[]}
      ]
  }" \
"https://app.datadoghq.com/api/v1/series?api_key=$DATADOG_API_KEY"
