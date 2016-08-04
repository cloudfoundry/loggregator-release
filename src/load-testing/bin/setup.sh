#!/bin/bash -e

export API_URL=`cf api | awk '{print $3}'`
export DOPPLER_URL=`cf curl /v2/info | jq --raw-output '.doppler_logging_endpoint'`
export UAA_URL=`cf curl /v2/info | jq --raw-output '.authorization_endpoint'`
echo -n "CF Client ID: "
read CLIENT_ID
echo -n "CF Client Secret: "
read -s CLIENT_SECRET
echo
export CLIENT_ID CLIENT_SECRET
echo -n "CF Username: "
read CF_USERNAME
echo -n "CF Password: "
read -s CF_PASSWORD
echo
echo -n "Rate: "
read RATE
echo -n "Time: "
read TIME
echo -n "Emitter Instances: "
read EMITTER_INSTANCES
echo -n "Counter Instances: "
read COUNTER_INSTANCES
echo -n "Org: "
read ORG
echo -n "Space: "
read SPACE
echo -n "Scheme: "
read SCHEME
export CF_USERNAME CF_PASSWORD RATE TIME EMITTER_INSTANCES COUNTER_INSTANCES ORG SPACE SCHEME
