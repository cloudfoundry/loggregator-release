# Cloud Native Logging [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://cloudfoundry.slack.com/archives/loggregator)

Loggregator is a [BOSH](http://bosh.io/) release deployed as a part of [cf-release](https://github.com/cloudfoundry/cf-release). Loggregator provides a highly-available (HA) and secure stream of logs and metrics for all applications and components on Cloud Foundry. It does so while not disrupting the behavior of the the applications and components on the platform (i.e. "backpressure").

The [Loggregator Design Notes](docs/loggregator-design.md) presents an overview of Loggregator components and architecture.

* [Generating TLS Certificates](#generating-tls-certificates)
* [Streaming Application Logs](#streaming-application-logs)
  * [Using the CF CLI](#using-the-cf-cli)
  * [Forwarding to a Log Drain](#forwarding-to-a-log-drain)
* [Consuming the Firehose](#consuming-the-firehose)
  * [User Account and Authentication Scope](#user-account-and-authentication-scope)
  * [Nozzle Development](#nozzle-development)
  * [Metrics](#metrics)
* [Emitting Logs and Metrics into Loggregator](#emitting-logs-and-metrics-into-loggregator)
  * [Loggregator API](#loggregator-api)
  * [Including Metron](#including-metron)
  * [Statsd-injector](#statsd-injector)
  * [Syslog-release](#syslog-release)
  * [Bosh HM forwarder](#bosh-hm-forwarder)
* [Tools for Testing and Monitoring Loggregator](#tools-for-testing-and-monitoring-loggregator)
  * [Tools](#tools)
  * [Health Nozzle](#health-nozzle)
* [More Resources and Documentation](#more-resources-and-documentation)



## Generating TLS Certificates
In order to secure transport of logs throughout the system, Loggregator needs several certificates to be generated for each of the components. You start this process by running the `scripts/generate-certs`, and then configure the corresponding certificates for each of the components you are deploying. For more details see our [Certificate Configuration README](docs/cert-config.md)

## Streaming Application Logs

Any user of Cloud Foundry can experience Loggregator by using two simple interfaces for streaming application specific logs. These do not require any special [User Account and Authentication(UAA)](https://github.com/cloudfoundry/uaa) Scope.

### Using the CF CLI
The fastest way to see your logs is by running the `cf logs` command using the [CF CLI](https://github.com/cloudfoundry/cli). Check the [Cloud Foundry CLI docs](http://cli.cloudfoundry.org/en-US/cf/logs.html) for more details.

### Forwarding to a Log Drain
If you’d like to save all logs for an application in a third party or custom tool that expects the syslog format, a log drain allows you to do so. Check the [Cloud Foundry docs](https://docs.cloudfoundry.org/devguide/services/log-management.html) for more details.

## Consuming the Firehose

The firehose is an aggregated stream of all application logs and component metrics on the platform. This allows operators to ensure they capture all logs within a microservice architecture as well as monitor the health of their platform.

### User Account and Authentication Scope
In order to consume the firehose you’ll need the `doppler.firehose` scope from UAA. For more details see this [Configuring the Firehose README](docs/configuring-firehose.md).

### Nozzle Development
Once you have configured appropriate authentication scope you are ready to start developing a nozzle for the firehose. See our [Nozzle community page](docs/community-nozzles.md) for more details about existing nozzles and how to get started.

### Metrics

Loggregator and other Cloud Foundry components emit regular messages through the Firehose that monitor the health, throughput, and details of a component's operations. For more detials about Loggregator’s metrics see our [Loggregator Metrics README](docs/metric_descriptions.md).

## Emitting Logs and Metrics into Loggregator
For components of Cloud Foundry or standalone BOSH deployments, Loggregator provides a set of tools for emitting Logs and Metrics.

### Loggregator API
The Loggregator API is a replacement of the [Dropsonde Protocol](https://github.com/cloudfoundry/dropsonde-protocol). Loggregator API defines an envelope structure which packages logs and metrics in a common format for distribution throughout Loggregator. See the [Loggregator API README](https://github.com/cloudfoundry/loggregator-api/blob/master/README.md) for more details.

### Including Metron
The Metron Agent listens on both UDP and gRPC endpoints for multiple versions of Loggregator API which it forwards onto the Firehose. To include Metron in your component or deployment see the [Setting up up Metron README](docs/metron.md).

### Statsd-injector
The statsd-injector is a companion component to Metron and allows use of the [statsd metric aggregator format](https://codeascraft.com/2011/02/15/measure-anything-measure-everything/). For more see the [statsd-injector README](https://github.com/cloudfoundry/statsd-injector/blob/master/README.md).

### Syslog Release
For some components (such as UAA) it makes sense to route logs separate from the Firehose. The syslog release using rsyslog to accomplish this. For more information see the [syslog-release README](https://github.com/cloudfoundry/syslog-release/blob/master/README.md) (note this release is maintianed by the bosh team).

### Bosh HM Forwarder
The Bosh Health Monitor Forwarder allows operators to capture health metrics from the Bosh Director through the Firehose. For more information see the [bosh-hm-forwarder README](https://github.com/cloudfoundry/bosh-hm-forwarder/blob/master/README.md).

## Tools for Testing and Monitoring Loggregator
### Tools
Loggregator provides a set of tools written in golang for testing the performance and efficacy of your loggregator installation. These tools emit logs and metrics in a controlled fashion. For more information see the [Loggregator tools README](docs/loggregator-tools.md).

### Health Nozzle
The Loggregator Team is currently proposing supporting a nozzle which aggregates health metrics across various Loggregator releases to monitor ingress, egress, and loss within the Loggregator system. See the [Health Nozzle Feature Proposal](https://docs.google.com/document/d/1rqlSDssaNk7B9TUmHhjUsn1-FeUNX8odslc-T_3ixck/edit) for more detials.

## More Resources and Documentation
### Roadmap
We communicate our long term planning using a [Product Road Map](https://docs.google.com/spreadsheets/d/1bM1bInPQeC2xLayLsFb0aBuD3_HFNfJj9mEJZygnuWo/edit#gid=0), and are always looking to gather feedback and input from Loggregator operators. Get in touch or file an issue if you have feature suggestions you'd like to see added.

### Pivotal Tracker
Items marked as "In Flight" on the Roadmap are tracked as new Features in [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/993188).
