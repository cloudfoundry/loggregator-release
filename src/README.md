# Loggregator [![slack.cloudfoundry.org][slack-badge]][loggregator-slack] [![CI Badge][ci-badge]][ci-pipeline]

Loggregator is the logging system used in [CloudFoundry][cf-deployment].

### Loggregator Goals

- Real time streaming of logs
- Producers do not experience backpressure
- Logs can be routed to several consumers
- Elastic and horizontal scalability
- High message reliability
- Low latency
- Flexibile consumption
- Security via gRPC with mutual TLS
- Opinionated log structure

### Known Limitations

- Logs are ephemeral/non-replicated within system
- No guaranteed delivery
- Limited persistence/querying ability

### Loggregator's Log Types

Loggregator uses Google's [protocol buffers][protobuf] along with [gRPC][grpc] to deliver logs. Loggregator has defined (via [Loggregator API][loggregator-api]) log types that are contained in a single protocol buffer type, named an `envelope`. Those types are:
- Log
- Counter
- Gauge
- Timer
- Event

##### Asynchronous vs Synchronous Data

The Loggregator system as a whole does not have an opinion about how frequently any log type is emitted. However, there are recommendations. `Counter` and `Gauge` logs should be emitted regularly (e.g., once a minute) so downstream consumers can easily plot the increasing total.

`Log`, `Timer` and `Event` should be emitted when the corresponding action occurred and therefore should be treated as asynchronous data. These data types do not lend themselves to be plotted as a time series.

### Loggregator Architecture

Loggregator is made up of 4 components:
- Agent
- Router
- ReverseLogProxy (RLP)
- TrafficController (TC)

##### Agent

The Agent is a daemon process that is intended to run on each container/VM. It is the entry point into Loggregator. Any log that is written to Loggregator is written to the Agent.

##### Router

The Router takes each log and publishes it to any interested consumer. Every log is in flight, and not available for a consumer if the consumer was late to connect. It routes envelopes via the [pubsub][pubsub] library that allows a consumer to give the Router "selectors" to whitelist what logs it wants. A consumer does not connect directly to a router.

##### Reverse Log Proxy

The ReverseLogProxy (RLP) has gRPC endpoints to allow consumers to read data from Loggregator. Each RLP connects to every  Router to ensure it gets all the relevant logs. Each Router only gets a subset of the logs, and therefore without the RLP, a consumer would only get a fraction of the logs. The RLP only speaks V2.

##### Reverse Log Proxy Gateway

The ReverseLogProxy (RLP) Gateway exposes the RLP API over HTTP.

The API Documentation can be found [here](./docs/rlp_gateway.md)

##### Traffic Controller

The TrafficController (TC) is like the RLP, but is tuned for CloudFoundry. It authenticates via the UAA and CloudController, which are both CF components. It egresses logs via Websockets. It only speaks V1. Planned deprecation in 2018.

### Avoiding Producer Backpressure

Loggregator chooses to drop logs instead of pushing back on producers. It does so with the [diodes][diodes] library. Therefore if anything upstream slows down, the diode will drop older messages. This allows producers to be resilient to upstream problems.

### Using Loggregator

##### go-loggregator

There is Go client library: [go-loggregator][go-loggregator]. The client library has several useful patterns along with examples to interact with a Loggregator system.

##### Deploying Loggregator

The most common way to deploy Loggregator is via [Bosh][bosh]. There is a [Loggregator Bosh release][loggregator-release] that is used heavily in a plethora of production environments. It is battle tested and well maintained.

[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]:        https://cloudfoundry.slack.com/archives/loggregator
[ci-badge]:                 https://loggregator.ci.cf-app.com/api/v1/pipelines/loggregator/jobs/loggregator-tests/badge
[ci-pipeline]:              https://loggregator.ci.cf-app.com/teams/main/pipelines/loggregator
[cf-deployment]:            https://github.com/cloudfoundry/cf-deployment
[protobuf]:                 https://developers.google.com/protocol-buffers/
[grpc]:                     https://grpc.io/
[loggregator-api]:          https://github.com/cloudfoundry/loggregator-api
[pubsub]:                   https://github.com/cloudfoundry/go-pubsub
[diodes]:                   https://github.com/cloudfoundry/go-diodes
[go-loggregator]:           https://github.com/cloudfoundry/go-loggregator
[bosh]:                     https://bosh.io
[loggregator-release]:      https://github.com/cloudfoundry/loggregator-release
