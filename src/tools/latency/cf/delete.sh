#!/bin/bash

app_name=${1:-loggregator-latency}
cf delete -f "$app_name"
