#!/bin/bash -ex
export API_ADDR="http://api.pecan.cf-app.com"
export DOPPLER_ADDR=`cf curl /v2/info | jq --raw-output '.doppler_logging_endpoint'`
export UAA_ADDR=`cf curl /v2/info | jq --raw-output '.authorization_endpoint'`
export CLIENT_ID="cf"
export CLIENT_SECRET=""
export USERNAME="[CF API USERNAME]"
export PASSWORD="[CF API PASSWORD]"
export MESSAGE_FILTER="logspinner"
export MESSAGES_PER_APP="2400000"
