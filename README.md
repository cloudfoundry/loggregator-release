# Cloud Native Logging [![slack.cloudfoundry.org][slack-badge]][loggregator-slack] [![CI Badge][ci-badge]][ci-pipeline]
asdfasdfasdf
Loggregator is a [BOSH][bosh] release deployed as a part of
[cf-release][cf-release]. Loggregator provides
a highly-available (HA) and secure stream of logs and metrics for all
applications and components on Cloud Foundry. It does so while not disrupting
the behavior of the the applications and components on the platform (i.e.
"backpressure").

The [Loggregator Design Notes](docs/loggregator-design.md) presents an
overview of Loggregator components and architecture.

* [Generating TLS Certificates](#generating-tls-certificates)
* [Streaming Application Logs](#streaming-application-logs)
  * [Using the CF CLI](#using-the-cf-cli)
  * [Forwarding to a Log Drain](#forwarding-to-a-log-drain)
  * [Log Ordering](#log-ordering)
* [Consuming the Firehose](#consuming-the-firehose)
  * [User Account and Authentication Scope](#user-account-and-authentication-scope)
  * [Nozzle Development](#nozzle-development)
  * [Metrics](#metrics)
* [Emitting Logs and Metrics into Loggregator](#emitting-logs-and-metrics-into-loggregator)
  * [Loggregator API](#loggregator-api)
  * [Including Metron](#including-metron)
  * [Statsd-injector](#statsd-injector)
  * [Syslog-release](#syslog-release)
* [Tools for Testing and Monitoring Loggregator](#tools-for-testing-and-monitoring-loggregator)
  * [Tools](#tools)
  * [Operator Guidebook](#operator-guidebook)
* [Troubleshooting Reliability](#troubleshooting-reliability)
  * [Scaling](#scaling)
  * [Noise](#noise)
* [More Resources and Documentation](#more-resources-and-documentation)

## Generating TLS Certificates

In order to secure transport of logs throughout the system, Loggregator needs
several certificates to be generated for each of the components. You start
this process by running the `scripts/generate-certs`, and then configure the
corresponding certificates for each of the components you are deploying. For
more details see our [Certificate Configuration README](docs/cert-config.md)

## Streaming Application Logs

Any user of Cloud Foundry can experience Loggregator by using two simple
interfaces for streaming application specific logs. These do not require any
special [User Account and Authentication(UAA)][uaa] Scope.

### Using the CF CLI

The fastest way to see your logs is by running the `cf logs` command using the
[CF CLI][cli]. Check the [Cloud Foundry CLI docs][cli-docs] for more details.

### Forwarding to a Log Drain

If you’d like to save all logs for an application in a third party or custom
tool that expects the syslog format, a log drain allows you to do so. Check
the [Cloud Foundry docs][cf-docs] for more details.

### Log Ordering

Loggregator does not provide any guaruntees around the order of delivery
of logs in streams. That said, there is enough precision in the timestamp provided
by diego that streaming clients can batch and order streams as they receive them.
This is done by the cf cli and most other streaming clients.

For syslog ingestion it also possible to ensure log order. See the details in [cf-syslog-drain-release](https://github.com/cloudfoundry/cf-syslog-drain-release/blob/develop/README.md#log-ordering)


## Consuming the Firehose

The firehose is an aggregated stream of all application logs and component
metrics on the platform. This allows operators to ensure they capture all logs
within a microservice architecture as well as monitor the health of their
platform. See the [Firehose README](docs/firehose.md).

### User Account and Authentication Scope

In order to consume the firehose you’ll need the `doppler.firehose` scope from
UAA. For more details see the [Firehose README](docs/firehose.md).

### Nozzle Development

Once you have configured appropriate authentication scope you are ready to
start developing a nozzle for the firehose. See our [Nozzle community
page](docs/community-nozzles.md) for more details about existing nozzles and
how to get started.

### Metrics

Loggregator and other Cloud Foundry components emit regular messages through
the Firehose that monitor the health, throughput, and details of a component's
operations. For more detials about Loggregator’s metrics see our [Loggregator
Metrics README](docs/metric_descriptions.md).

## Emitting Logs and Metrics into Loggregator

For components of Cloud Foundry or standalone BOSH deployments, Loggregator
provides a set of tools for emitting Logs and Metrics.

## Reverse Log Proxy (RLP)
The RLP is the v2 implementation of the [Loggregator API][api-readme]. This
component is intended to be a replacement for traffic controller.

### RLP Gateway
By default, the RLP communicates with clients via gRPC over mutual TLS. To enable HTTP access to the Reverse Log
Proxy, deploy the RLP Gateway. The RLP Gateway API is documented
[in the Loggregatgor repository][loggregator]


## Standalone Loggregator

Standalone Loggregator provides logging support for services that do not
require a CloudFoundry.

TrafficController has several CloudFoundry concerns. Therefore, there is an
[operations file](./manifests/operations/no-traffic-controller.yml) to deploy
standalone Loggregator without it.

### Deploying with TrafficController

##### Example bosh lite deploy

```
bosh -e lite deploy -d loggregator manifests/loggregator.yml \
    --vars-store=/tmp/loggregator-vars.yml
```

### Loggregator API

The Loggregator API is a replacement of the [Dropsonde
Protocol][dropsonde-protocol]. Loggregator API defines an envelope structure
which packages logs and metrics in a common format for distribution throughout
Loggregator. See the [Loggregator API README][api-readme] for more details.

### Including Metron

The Metron Agent listens on both UDP and gRPC endpoints for multiple versions
of Loggregator API which it forwards onto the Firehose. To include Metron in
your component or deployment see the [Setting up up Metron
README](docs/metron.md).

### Statsd-injector

The statsd-injector is a companion component to Metron and allows use of the
[statsd metric aggregator format][statsd-format]. For more see the
[statsd-injector README][statsd-injector-readme].

### Syslog Release

For some components (such as UAA) it makes sense to route logs separate from
the Firehose. The syslog release using rsyslog to accomplish this. For more
information see the [syslog-release README][syslog-release-readme] (note this
release is maintianed by the bosh team).

## Tools for Testing and Monitoring Loggregator

### Tools

Loggregator provides a set of tools for testing the
performance and reliability of your loggregator installation.
See the [loggregator tools](http://code.cloudfoundry.org/loggregator-tools)
repo for more details.

### Operator Guidebook
The [Loggregator Operator Guidebook](./docs/Loggregator%20Operator%20Guide.pdf) provides details for scaling
and managing Loggregator along with detailed results of capacity
planning tests.

## Troubleshooting Reliability

### Scaling
In addition to the scaling recommendations above, it is important that
the resources for Loggregator are dedicate VM’s with similar footprints
to those used in our capacity tests. Even if you are within the bounds of
the scaling recommendations it may be useful to scale Loggregator and
Nozzle components aggressively to rule out scaling as a major cause log loss.

### Noise
Another common reason for log loss is due to an application producing a
large amount of logs that drown out the logs from other application on
the cell it is running on. To identify and monitor for this behavior the
Loggregator team has created a Noisy Neighbor Nozzle and CLI Tool. This
tool will help operators quickly identify and take action on noise
producing applications.  Instruction for deploying and using this nozzle
are in the repo.

## More Resources and Documentation

### Roadmap

We communicate our long term planning using a [Product Road Map](https://github.com/cloudfoundry/loggregator-release/projects/1),
and are always looking to gather feedback and input from Loggregator
operators. Get in touch or file an issue if you have feature suggestions you'd
like to see added.

### Pivotal Tracker

Items marked as "In Flight" on the Roadmap are tracked as new Features in
[Pivotal Tracker][loggregator-tracker].

[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]:        https://cloudfoundry.slack.com/archives/loggregator
[bosh]:                     https://bosh.io/
[cf-release]:               https://github.com/cloudfoundry/cf-release
[uaa]:                      https://github.com/cloudfoundry/uaa
[cli]:                      https://github.com/cloudfoundry/cli
[loggregator]:              https://code.cloudfoundry.org/loggregator
[cli-docs]:                 https://cli.cloudfoundry.org/en-US/cf/logs.html
[cf-docs]:                  https://docs.cloudfoundry.org/devguide/services/log-management.html
[dropsonde-protocol]:       https://github.com/cloudfoundry/dropsonde-protocol
[api-readme]:               https://github.com/cloudfoundry/loggregator-api/blob/master/README.md
[statsd-format]:            https://codeascraft.com/2011/02/15/measure-anything-measure-everything/
[statsd-inejctor-readme]:   https://github.com/cloudfoundry/statsd-injector/blob/master/README.md
[syslog-release-readme]:    https://github.com/cloudfoundry/syslog-release/blob/master/README.md
[health-nozzle-proposal]:   https://docs.google.com/document/d/1rqlSDssaNk7B9TUmHhjUsn1-FeUNX8odslc-T_3ixck/edit
[road-map]:                 https://docs.google.com/spreadsheets/d/1bM1bInPQeC2xLayLsFb0aBuD3_HFNfJj9mEJZygnuWo/edit#gid=0
[loggregator-tracker]:      https://www.pivotaltracker.com/n/projects/993188
[ci-badge]:                 https://loggregator.ci.cf-app.com/api/v1/pipelines/loggregator/jobs/loggregator-tests/badge
[ci-pipeline]:              https://loggregator.ci.cf-app.com/teams/main/pipelines/loggregator
