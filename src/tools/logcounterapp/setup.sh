#!/bin/bash -e

export MESSAGE_PREFIX="logemitter"
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
echo -n "Subscription Id: "
read SUBSCRIPTION_ID
export CF_USERNAME CF_PASSWORD SUBSCRIPTION_ID
