# Metron

Metron is a Cloud Foundry component that forwards logs and metrics into the Loggregator subsystem. Within Loggregator, the role of Metron is to take traffic from the various emitter sources (dea, dea-logging-agent, router, etc) and route that traffic to one or more [dopplers](../doppler). An instance of Metron runs on each VM in an environment and is therefore co-located on the logging agents that run on the Cloud Foundry components (i.e. sources).

In the current configuration, traffic is routed by default to Dopplers in the same AZ, but it can fall back to any doppler if none are available in the current AZ. All Metron traffic is randomly distributed across available dopplers.

Metron only listens to local network interfaces and all logs and metrics are immediately signed before forwarding to Dopplers. This prevents man-in-the-middle attacks and ensures data integrity.

## Architecture Within Loggregator

![Loggregator Diagram](../../docs/loggregator.png)

## Usage
```metron [--logFile <path to log file>] [--config <path to config file>] [--debug=<true|false>]```

| Flag      | Required                              | Description                                     |
|-----------|---------------------------------------|-------------------------------------------------|
| --logFile | No, default: STDOUT                   | The agent log file.                             |
| --config  | No, default: ```config/metron.json``` | Location of the Metron configuration JSON file. |
| --debug   | No, default: ```false```              | Debug logging                                   |

## Editing Manifest Templates
Currently the Doppler/Metron manifest configuration lives [here](../../manifest-templates/cf-lamb.yml). Editing this file will make changes in the [manifest templates](https://github.com/cloudfoundry/cf-release/tree/master/templates) in cf-release. When making changes to these templates, you should be working out of the loggregator submodule in cf-release. After changing this configuration, you will need to run the tests in root directory of cf-release with `bundle exec rspec`. These tests will pull values from [lamb-properties](../../manifest-templates/lamb-properties.rb) in order to populate the fixtures. Necessary changes should be made in [lamb-properties](../../manifest-templates/lamb-properties.rb).

## Benchmark tests

https://gist.github.com/jmtuley/7a376e2d5ecd9569d1d1
