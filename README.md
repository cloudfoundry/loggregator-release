# Loggregator Release

Loggregator is a [BOSH][bosh] release deployed as a part of
[cf-deployment][cf-deployment]. Loggregator provides
a highly-available (HA) and secure stream of logs and metrics for all
applications and components on Cloud Foundry. It does so while not disrupting
the behavior of the the applications and components on the platform (i.e.
"backpressure").

The [Loggregator Design Notes](docs/loggregator-design.md) presents an
overview of Loggregator components and architecture.

If you have any questions, or want to get attention for a PR or issue please reach out on the [#logging-and-metrics channel in the cloudfoundry slack](https://cloudfoundry.slack.com/archives/CUW93AF3M)

## Table of Contents

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
  * [Loggregator Agents](#loggregator-agents)
  * [Statsd-injector](#statsd-injector)
  * [Syslog-release](#syslog-release)
* [Tools for Testing and Monitoring Loggregator](#tools-for-testing-and-monitoring-loggregator)
* [Troubleshooting Reliability](#troubleshooting-reliability)
  * [Scaling](#scaling)
  * [Noise](#noise)

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

Loggregator does not provide any guarantees around the order of delivery
of logs in streams. That said, there is enough precision in the timestamp provided
by diego that streaming clients can batch and order streams as they receive them.
This is done by the cf cli and most other streaming clients.

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
Proxy, deploy the RLP Gateway.

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

### Loggregator Agents

Loggregator Agents receive logs and metrics on VMs, and forward them onto the
Firehose. For more info see the [loggregator-agent release][loggregator-agent-release].

### Statsd-injector

The statsd-injector receives metrics from components in the
[statsd metric aggregator format][statsd-format]. For more info see the
[statsd-injector README][statsd-injector-readme].

### Syslog Release

For some components (such as UAA) it makes sense to route logs separate from
the Firehose. The syslog release uses rsyslog to accomplish this. For more
information see the [syslog-release README][syslog-release-readme].

## Tools for Testing and Monitoring Loggregator

Loggregator provides a set of tools for testing the
performance and reliability of your loggregator installation.
See the [loggregator tools](http://code.cloudfoundry.org/loggregator-tools)
repo for more details.

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

[bosh]:                      https://bosh.io/
[cf-deployment]:             https://github.com/cloudfoundry/cf-deployment
[uaa]:                       https://github.com/cloudfoundry/uaa
[cli]:                       https://github.com/cloudfoundry/cli
[cli-docs]:                  https://cli.cloudfoundry.org/en-US/cf/logs.html
[cf-docs]:                   https://docs.cloudfoundry.org/devguide/services/log-management.html
[dropsonde-protocol]:        https://github.com/cloudfoundry/dropsonde-protocol
[api-readme]:                https://github.com/cloudfoundry/loggregator-api/blob/master/README.md
[statsd-format]:             https://codeascraft.com/2011/02/15/measure-anything-measure-everything/
[statsd-injector-readme]:    https://github.com/cloudfoundry/statsd-injector/blob/master/README.md
[syslog-release-readme]:     https://github.com/cloudfoundry/syslog-release/blob/master/README.md
[loggregator-agent-release]: https://github.com/cloudfoundry/loggregator-agent-release
