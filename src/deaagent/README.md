# DEA Logging Agent

DEA Logging Agent is a Cloud Foundry component that opens a STDOUT and STDERR socket connection to a DEA container, reads application logs from these sockets, and forwards them to the [Metron agent](../metron).

## Usage
```
deaagent [--logFile <path to log file>] [--config <path to config file>] \
    [--debug=<true|false>] [--cpuprofile <path to desired CPU profile>] \
    [--memprofile <path to desired memory profile>] \
    [--instancesFile <path to DEA instances file>]
```

| Flag               | Required                               | Description                                     |
|--------------------|----------------------------------------|-------------------------------------------------|
| ```--logFile```    | No, default: STDOUT                    | The agent log file.                             |
| ```--config```     | No, default: ```config/dea_logging_agent.json``` | Location of the DEA Logging Agent configuration JSON file. |
| ```--debug```      | No, default: ```false```               | Debug logging                                   |
| ```--cpuprofile``` | No, default: no CPU profiling          | Write CPU profile to a file.                    |
| ```--memprofile``` | No, default: no memory profiling       | Write memory profile to a file.                 |
| ```--instancesFile```| No, default: ```/var/vcap/data/dea_next/db/instances.json``` | The DEA instances JSON file |

## Editing Manifest Templates
Currently the DEA Logging Agent manifest configuration lives [here](../../manifest-templates/cf-lamb.yml). Editing this file will make changes in the [manifest templates](https://github.com/cloudfoundry/cf-release/tree/master/templates) in cf-release. When making changes to these templates, you should be working out of the loggregator submodule in cf-release. After changing this configuration, you will need to run the tests in root directory of cf-release with `bundle exec rspec`. These tests will pull values from [lamb-properties](../../manifest-templates/lamb-properties.rb) in order to populate the fixtures. Necessary changes should be made in [lamb-properties](../../manifest-templates/lamb-properties.rb).