# Metron

Metron is a Cloud Foundry component that forwards logs and metrics into the Loggregator subsystem by taking traffic from the various emitter sources (dea, dea-logging-agent, router, etc) and routing that traffic to one or more [dopplers](../doppler). An instance of Metron runs on each VM in an environment and is therefore co-located on the emitter sources.

Traffic is routed to Dopplers in the same AZ, but it can fall back to any Doppler if none are available in the current AZ. All Metron traffic is randomly distributed across available Dopplers. Metron keeps track of healthy dopplers by polling etcd for their health status.

Metron only listens to local network interfaces and all logs and metrics are immediately signed before forwarding to Dopplers. This prevents man-in-the-middle attacks and ensures data integrity.

## Architecture Within Loggregator

![Loggregator Diagram](../../docs/metron.png)

Source agents emit the logging data through the system as [protocol-buffers](https://developers.google.com/protocol-buffers/) via the [Dropsonde Protocol](https://github.com/cloudfoundry/dropsonde-protocol). Metrics can also be emitted using statsd. The statsd metrics are forwarded to Metron by the [statsd-injector](https://github.com/cloudfoundry/statsd-injector)

## Usage
```metron [--logFile <path to log file>] [--config <path to config file>] [--debug=<true|false>]```

| Flag            | Required                              | Description                                     |
|-----------------|---------------------------------------|-------------------------------------------------|
| ```--logFile``` | No, default: STDOUT                   | The agent log file.                             |
| ```--config```  | No, default: ```config/metron.json``` | Location of the Metron configuration JSON file. |
| ```--debug```   | No, default: ```false```              | Debug logging                                   |

## Editing Manifest Templates
The up-to-date Metron configuration can be found [in the metron spec file](../../jobs/metron_agent/spec). You can see a list of available configurable properties, their defaults and descriptions in that file. 

## Benchmark tests

[loggregator/src/tools/metronbenchmark](https://github.com/cloudfoundry/loggregator/tree/develop/src/tools/metronbenchmark)
