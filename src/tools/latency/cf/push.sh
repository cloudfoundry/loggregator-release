#!/bin/bash

set -e

function target_url {
    cf curl /v2/info | \
        python -c 'import json, sys; print json.load(sys.stdin).get("doppler_logging_endpoint")'
}
app_name=${1:-loggregator-latency}
cf push "$app_name" -b binary_buildpack --no-start -m 64M -c ./latency
cf set-env "$app_name" TARGET_URL "$(target_url)"
cf set-env "$app_name" TOKEN "$(cf oauth-token)"
cf start "$app_name"
