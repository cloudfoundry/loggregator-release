# Doppler

Doppler is a Cloud Foundry component that gathers logs and metrics from [Metron agents](../metron), stores them into temporary buffers, and forwards them to third-party syslog drains and other consumers.

Doppler scales horizontally &mdash; as load increases, more instances can be deployed to spread the load and decrease memory usage, CPU, etc. per Doppler. Metron agents are notified of the additional instances via updates to ```etcd```.

## Architecture Within Loggregator

![Loggregator Diagram](../../docs/loggregator.png)

Logging data passes through the system as [protocol-buffers](https://github.com/google/protobuf), using [Dropsonde](https://github.com/cloudfoundry/dropsonde).

In a redundant Cloud Foundry setup, Loggregator can be configured to survive zone failures. Log messages from non-affected zones will still make it to the end user. On AWS, availability zones could be used as redundancy zones. The following is an example of a multi zone setup with two zones.

![Loggregator Diagram](../../docs/loggregator_multizone.png)

## Usage
```
doppler [--logFile <path to log file>] [--config <path to config file>] \
    [--debug=<true|false>] [--cpuprofile <path to desired CPU profile>] \
    [--memprofile <path to desired memory profile>]
```

| Flag               | Required                               | Description                                     |
|--------------------|----------------------------------------|-------------------------------------------------|
| ```--logFile```    | No, default: STDOUT                    | The agent log file.                             |
| ```--config```     | No, default: ```config/doppler.json``` | Location of the Doppler configuration JSON file. |
| ```--debug```      | No, default: ```false```               | Debug logging                                   |
| ```--cpuprofile``` | No, default: no CPU profiling          | Write CPU profile to a file.                    |
| ```--memprofile``` | No, default: no memory profiling       | Write memory profile to a file.                 |

## Emitting Messages from the other Cloud Foundry components

Cloud Foundry developers can easily add source clients to new CF components that emit messages to Doppler.  Currently, there are libraries for [Go](https://github.com/cloudfoundry/dropsonde/) and [Ruby](https://github.com/cloudfoundry/loggregator_emitter). For usage information, look at their respective READMEs.

## Editing Manifest Templates
Currently the Doppler manifest configuration lives [here](../../manifest-templates/cf-lamb.yml). Editing this file will make changes in the [manifest templates](https://github.com/cloudfoundry/cf-release/tree/master/templates) in cf-release. When making changes to these templates, you should be working out of the loggregator submodule in cf-release. After changing this configuration, you will need to run the tests in root directory of cf-release with `bundle exec rspec`. These tests will pull values from [lamb-properties](../../manifest-templates/lamb-properties.rb) in order to populate the fixtures. Necessary changes should be made in [lamb-properties](../../manifest-templates/lamb-properties.rb).