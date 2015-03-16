# Loggregator

[![Build Status](https://travis-ci.org/cloudfoundry/loggregator.svg?branch=develop)](https://travis-ci.org/cloudfoundry/loggregator)  [![Coverage Status](https://coveralls.io/repos/cloudfoundry/loggregator/badge.png?branch=develop)](https://coveralls.io/r/cloudfoundry/loggregator?branch=develop)
   
### Logging in the Clouds   
  
Loggregator is the user application logging subsystem of Cloud Foundry.

### Features

Loggregator allows users to:

1. Tail their application logs.
1. Dump a recent set of application logs (where recent is a configurable number of log packets).
1. Continually drain their application logs to 3rd party log archive and analysis services.
1. (Operators and administrators only) Access the firehose, which includes the combined stream of logs from all apps, plus metrics data from CF components.

### Usage

First, make sure you're using the new [golang based CF CLI](https://github.com/cloudfoundry/cli).  Once that's installed:

```
cf logs APP_NAME [--recent]
```
For example:

``` bash
$ cf logs private-app
Connected, tailing...
Oct 3 15:09:26 private-app App/0 STDERR This message is on stderr at 2013-10-03 22:09:26 +0000 for private-app instance 0
Oct 3 15:09:26 private-app App/0 STDERR 204.15.2.45, 10.10.2.148 - - [03/Oct/2013 22:09:26] "GET / HTTP/1.1" 200 81 0.0010
Oct 3 15:09:26 private-app App/0 This message is on stdout at 2013-10-03 22:09:26 +0000 for private-app instance 0
Oct 3 15:09:26 private-app App/0 STDERR This message is on stderr at 2013-10-03 22:09:26 +0000 for private-app instance 0
^C
```

### Constraints

1. Loggregator collects STDOUT & STDERR from applications.  This may require configuration on the developer's side.
1. A Loggregator outage must not affect the running application.
1. Loggregator gathers and stores logs in a best-effort manner.  While undesirable, losing the current buffer of application logs is acceptable.
1. The 3rd party drain API should mimic Heroku's in order to reduce integration effort for our partners.  The Heroku drain API is simply remote syslog over TCP.

### Architecture

Loggregator is composed of:

* **Sources**: Logging agents that run on the Cloud Foundry components.
* **Metron**: Metron agents are co-located with sources. They collect logs and forward them to:
* **Doppler**: Responsible for gathering logs from the **Metron agents**, storing them in temporary buffers, and forwarding logs to 3rd party syslog drains.
* **Traffic Controller**: Handles client requests for logs. Gathers and collates messages from all Doppler servers, and provides external API and message translation (as needed for legacy APIs).

Source agents emit the logging data as [protocol-buffers](https://code.google.com/p/protobuf/), and the data stays in that format throughout the system.

![Loggregator Diagram](docs/loggregator.png)

In a redundant CloudFoundry setup, Loggregator can be configured to survive zone failures. Log messages from non-affected zones will still make it to the end user. On AWS, availability zones could be used as redundancy zones. The following is an example of a multi zone setup with two zones.

![Loggregator Diagram](docs/loggregator_multizone.png)

The role of Metron is to take traffic from the various emitter sources (dea, dea-logging-agent, router, etc) and route that traffic to one or more dopplers. In the current config we route this traffic to the dopplers in the same az. The traffic is randomly distributed across dopplers.

The role of Traffic Controller is to handle inbound HTTP and WebSocket requests for log data. It does this by proxying the request to all dopplers (regardless of AZ). Since an application can be deployed to multiple AZs, its logs can potentially end up on dopplers in multiple AZs. This is why the traffic controller will attempt to connect to dopplers in each AZ and will collate the data into a single stream for the web socket client.

The traffic controller itself is stateless; an incoming request can be handled by any instance in any AZ.

Traffic controllers also exposes a `firehose` web socket endpoint. Connecting to this endpoint establishes connections to all dopplers, and streams logs and metrics for all applications and CF components.

### Emitting Messages from other Cloud Foundry components

Cloud Foundry developers can easily add source clients to new CF components that emit messages to the doppler.  Currently, there are libraries for [Go](https://github.com/cloudfoundry/dropsonde/) and [Ruby](https://github.com/cloudfoundry/loggregator_emitter). For usage information, look at their respective READMEs.

### Deploying via BOSH

Below are example snippets for deploying the DEA Logging Agent (source), Doppler, and Loggregator Traffic Controller via BOSH.

```yaml
jobs:
- name: dea_next
  templates:
  - name: dea_next
    release: cf
  - name: dea_logging_agent
    release: cf
  - name: metron_agent
    release: cf
  instances: 1
  resource_pool: dea
  networks:
  - name: cf1
    default:
    - dns
    - gateway
  properties:
    dea_next:
      zone: z1
    metron_agent:
      zone: z1
    networks:
      apps: cf1

- name: loggregator
  templates: 
  - name: doppler
    release: cf
  instances: 1  # Scale out as neccessary
  resource_pool: common
  networks:
  - name: cf1
  properties:
    doppler:
      zone: z1
    networks:
      apps: cf1

- name: loggregator_trafficcontroller
  templates: 
  - name: loggregator_trafficcontroller
    release: cf
  - name: metron_agent
    release: cf
  instances: 1  # Scale out as necessary
  resource_pool: common
  networks:
  - name: cf1
  properties:
    traffic_controller:
      zone: z1 # Denoting which one of the redundancy zones this traffic controller is servicing
    metron_agent:
      zone: z1
    networks:
      apps: cf1


properties:
  loggregator:
    servers:
      z1: # A list of loggregator servers for every redundancy zone
      - 10.10.16.14
    incoming_port: 3456
    outgoing_port: 8080
    
  loggregator_endpoint: # The end point sources will connect to
    shared_secret: loggregatorEndPointSharedSecret  
    host: 10.10.16.16
    port: 3456
```

### Configuring the Firehose

The firehose feature includes the combined stream of logs from all apps, plus metrics data from CF components, and is intended to be used by operators and adminstrators.

Access to the firehose requires a user with the `doppler.firehose` scope.

The "cf" UAA client needs permission to grant this custom scope to users.
The configuration of the `uaa` job in Cloud Foundry [adds this scope by default](https://github.com/cloudfoundry/cf-release/blob/2a3d95417da3c59564daeecd754eb00862030cd6/jobs/uaa/templates/uaa.yml.erb#L111).
However, if your Cloud Foundry instance overrides the `properties.uaa.clients.cf` property in a stub, you need to add `doppler.firehose` to the scope list in the `properties.uaa.clients.cf.scope` property.

#### Configuring at deployment time (via deployment manifest)
In your deployment manifest, add

```yaml
properties:
  …
  uaa:
    …
    clients:
      …
      cf:
        scope: …,doppler.firehose
      …
      doppler:
        override: true
        authorities: uaa.resource
        secret: YOUR-DOPPLER-SECRET
```

(The `properties.uaa.clients.doppler.id` key should be populated automatically.) These are also set by default in [cf-properties.yml](https://github.com/cloudfoundry/cf-release/blob/master/templates/cf-properties.yml#L304-L307).

#### Adding scope to a running cluster (via `uaac`)

Before continuing, you should be familiar with the [`uaac` tool](http://docs.cloudfoundry.org/adminguide/uaa-user-management.html).

1. Ensure that doppler is a UAA client. If `uaac client get doppler` returns output like

  ```
    scope: uaa.none
    client_id: doppler
    resource_ids: none
    authorized_grant_types: authorization_code refresh_token
    authorities: uaa.resource
  ```

  then you're set.

  1. If it does not exist, run `uaac client add doppler --scope uaa.none --authorized_grant_types authorization_code,refresh_token --authorities uaa.resource` (and set its secret).
  1. If it exists but with incorrect properties, run `uaac client update doppler --scope uaa.none --authorized_grant_types "authorization_code refresh_token" --authorities uaa.resource`.
1. Grant firehose access to the `cf` client.
  1. Check the scopes assigned to `cf` with `uaac client get cf`, e.g.

    ```
      scope: cloud_controller.admin cloud_controller.read cloud_controller.write openid password.write scim.read scim.userids scim.write
      client_id: cf
      resource_ids: none
      authorized_grant_types: implicit password refresh_token
      access_token_validity: 600
      refresh_token_validity: 2592000
      authorities: uaa.none
      autoapprove: true
    ```

  1. Copy the existing scope and add `doppler.firehose`, then update the client

    ```
    uaac client update cf --scope "cloud_controller.admin cloud_controller.read cloud_controller.write openid password.write scim.read scim.userids scim.write doppler.firehose"
    ```

### Consuming log and metric data

The [NOAA Client](https://github.com/cloudfoundry/noaa) library, written in Golang, can be used by Go applications to consume app log data as well as the log + metrics firehose. If you wish to write your own client application using this library, please refer to the NOAA source and documentation.

Multiple subscribers may connect to the firehose endpoint, each with a unique subscription_id. Each subscriber (in practice, a pool of clients with a common subscription_id) receives the entire stream. For each subscription_id, all data will be distributed evenly among that subscriber's client pool.

### Development

The Cloud Foundry team uses GitHub and accepts contributions via [pull request](https://help.github.com/articles/using-pull-requests).

Follow these steps to make a contribution to any of our open source repositories:

1. Complete our CLA Agreement for [individuals](http://www.cloudfoundry.org/individualcontribution.pdf) or [corporations](http://www.cloudfoundry.org/corpcontribution.pdf)
1. Set your name and email

    ```
    git config --global user.name "Firstname Lastname"
    git config --global user.email "your_email@youremail.com"
    ```

1. Fork the repo (from `develop` branch to get the latest changes)
1. Make your changes on a topic branch, commit, and push to github and open a pull request against the `develop` branch.

Once your commits are approved by Travis CI and reviewed by the core team, they will be merged.

#### OS X prerequisites

Use brew and do

    brew install go --cross-compile-all
    brew install direnv
    
Make sure you add the proper entry to load direnv into your shell. See `brew info direnv`
for details. To be safe, close the terminal window that you are using to make sure the 
changes to your shell are applied.
    
#### Checkout

```
git clone https://github.com/cloudfoundry/loggregator
cd loggregator # When you cd into the loggregator dir for the first time direnv will prompt you to trust the config file
git submodule update --init
```
Please run `bin/install-git-hooks` before committing for the first time. The pre-commit hook that this installs will ensure that all dependencies are properly listed in the `bosh/packages` directory. (Of course, you should probably convince yourself that the hooks are safe before installing them.) Without this script, it is possible to commit a version of the repository that will not compile.

#### Additional go tools

Install go vet and go cover

If Go version is less than Go 1.4 then 

    go get code.google.com/p/go.tools/cmd/vet
    go get code.google.com/p/go.tools/cmd/cover

else 

    go get golang.org/x/tools/cmd/vet
    go get golang.org/x/tools/cmd/cover

Install gosub
```
go get github.com/vito/gosub
```
#### Running tests

```
bin/test
```

#### Running specific tests

```
export GOPATH=`pwd` #in the root of the project
go get github.com/onsi/ginkgo/ginkgo
export PATH=$PATH:$GOPATH/bin
cd src/loggregator # or any other component
ginkgo -r
```

#### Debugging


Doppler will dump information about the running goroutines to stdout if sent a `USR1` signal.

```
goroutine 1 [running]:
runtime/pprof.writeGoroutineStacks(0xc2000bc3f0, 0xc200000008, 0xc200000001, 0xca0000c2001fcfc0)
	/home/travis/.gvm/gos/go1.1.1/src/pkg/runtime/pprof/pprof.go:511 +0x7a
runtime/pprof.writeGoroutine(0xc2000bc3f0, 0xc200000008, 0x2, 0xca74765c960d5c8f, 0x40bbf7, ...)
	/home/travis/.gvm/gos/go1.1.1/src/pkg/runtime/pprof/pprof.go:500 +0x3a
....
``` 

#### Editing Manifest Templates

Currently the Doppler/Metron manifest configuration lives [here](manifest-templates/cf-lamb.yml).
Editing this file will make changes in the [manifest templates](https://github.com/cloudfoundry/cf-release/tree/master/templates) in cf-release.
When making changes to these templates, you should be working out of the loggregator submodule in cf-release.
After changing this configuration, you will need to run the tests in root directory of cf-release with `bundle exec rspec`.
These tests will pull values from [lamb-properties](manifest-templates/lamb-properties.rb) in order to populate the fixtures.
Necessary changes should be made in [lamb-properties](manifest-templates/lamb-properties.rb).
