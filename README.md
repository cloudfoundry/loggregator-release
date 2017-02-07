### Cloud Native Logging 

Loggregator is a BOSH release deployed as a part of cf-release. Loggregator provides a highly-available (HA) and secure stream of logs and metrics for all applications and components on Cloud Foundry. It must do so while not disrupting the behavior of the the applications and components on the platform (i.e "backpressure").

The [Loggregator Design Notes]() presents an overview of Loggregator components, and links to Loggregator Components.

This README outlines resources and interfaces for Loggregator 

### Security Configurations
In order to secure transport of logs throughout the system, Loggregator needs several certificates to be generated for each of the components. You start this process by running the `/scripts/generate-certs`, and then configure the corresponding certificates for each of the components you are deploying. For more details see our [Certificate Configuration README]()

### Streaming Application Logs

Any user of CloudFoundry can experience Loggregator by using two simple interfaces for streaming application specific logs. These dno not require any special User Account and Authentication(UAA) Scope. 

## Using the CF CLI 
The fastest way to see your logs is by running the `cf logs` command. Check the [CloudFoundry docs]() for more details. 

## Forwarding to a Log Drain
If you’d like to save all logs for an application in a third party or custom tool that expects the syslog format a log drain allows you to do so. Check the [CloudFoundry docs]() for more details. 

### Consuming the Firehose

The firehose is an aggregated stream of all application logs and component metrics on the platform. This allows operators to ensure they capture all logs within a microservice architecture as well as monitor the health of their platform. 

## User Account and Authentication (UAA) Scope
In order to consume the firehose you’ll need the doppler firehose scope. For more details see this [Configuring the Firehose README]().

## Nozzle Development
Once you have configured appropriate authentication scope you are ready to start developing a nozzle for the firehose. See our [Nozzle community page]() for more details about existing nozzles and how to get started. 

## Metrics
Loggregator and other CloudFoundry components emit regular messages through the Firehose that monitor the health, throughput, and details of a component's operations. For more detials about Loggregator’s metrics see our Loggregator Metrics README.

### Emitting Logs and Metrics into Loggregator
For components of Cloud Foundry or a standalone BOSH deployments, Loggregator provides a set of tools for emitting Logs and Metrics. 

## Including Metron 
Metron listens on both UDP and gRPC endpoints for multiple versions of Loggregator API which it forwards onto the Firehose. To include Metron in your component or deployment see the [Settup up Metron README](). 

## Loggregator API
The Loggregator API (formerly known as Dropsonde) defines an envelope structure which packages logs and metrics in a common format for distribution throughout Loggregator. See the [Loggregator API README]() for more details. 

## Statsd-injector
The statsd-injector is a companion component to Metron and allows use of the statsd metric aggregator format. For more see the [stats-d-injector README]().

## Syslog Release
For some components (such as UAA) it makes sense to route logs separate from the Firehose. The syslog release using rsyslog to accomplish this. For more information see the [syslog-release README]() (note this release is maintianed by the bosh team).

## Bosh HM Forwarder
The bosh-HM Forwarder allows operators to capture health metrics from the Bosh Director through the Firehose. For more information see the [boshhmforwarder README](). 

### Tools for Testing and Monitoring Loggregator
## Tools
Loggregator provides a set of tools written in golang for testing the performance and efficacy of your loggregator installation. These tools emit logs and metrics in a controlled fashion. For more information see the [Loggregator tools README](). 

## Health Nozzle (proposed)
The Loggregator Team is currently proposing supporting a nozzle which aggregates health metrics across various Loggregator releases to monitor, ingress, egress, and loss within the Loggregator system. See the [Health Nozzle Feature Proposal]() for more detials. 

### More Resources and Documentation
## Cloud Foundry Docs
## Glossary
## Trouble Shooting Guides
## Roadmap
## Pivotal Tracker
