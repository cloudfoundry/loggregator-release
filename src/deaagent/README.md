# DEA Logging Agent

DEA Logging Agent is a Cloud Foundry component that opens a STDOUT and STDERR socket connection to a DEA container, reads application logs from these sockets, and forwards them to the [Metron agent](../metron). The DEA logging agent figures out the socket connections for a particular application by periodically reading the `instances.json` file.

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
The up-to-date DEA logging agent configuration can be found [in the dea_logging_agent spec file](../../jobs/dea_logging_agent/spec). You can see a list of available configurable properties, their defaults and descriptions in that file. 
