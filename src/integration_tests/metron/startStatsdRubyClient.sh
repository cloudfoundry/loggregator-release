#!/usr/bin/env bash

cd ~/workspace/loggregator/src/tools/statsdRubyClient/

bundle install

bundle exec ./statsdClient.rb 51162