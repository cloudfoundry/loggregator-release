# Loggregator 

[![Build Status](https://travis-ci.org/cloudfoundry/loggregator.svg?branch=develop)](https://travis-ci.org/cloudfoundry/loggregator)  [![Coverage Status](https://coveralls.io/repos/cloudfoundry/loggregator/badge.png?branch=develop)](https://coveralls.io/r/cloudfoundry/loggregator?branch=develop)
   
### Logging in the Clouds   
  
Loggregator is the user application logging subsystem for Cloud Foundry.    

### Features

Loggregator allows users to:

1. Tail their application logs.
1. Dump a recent set of application logs (where recent is a configurable number of log packets).
1. Continually drain their application logs to 3rd party log archive and analysis services.

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

1. Loggregator collects STDOUT & STDERR from the customer's application.  This may require configuration on the developer's side.
1. A Loggregator outage must not affect the running application.
1. Loggregator gathers and stores logs in a best-effort manner.  While undesirable, losing the current buffer of application logs is acceptable.
1. The 3rd party drain API should mimic Heroku's in order to reduce integration effort for our partners.  The Heroku drain API is simply remote syslog over TCP.

### Architecture

Loggregator is composed of:

* **Sources**: Logging agents that run on the Cloud Foundry components.  They forward logs to:
* **Loggregator Server**: Responsible for gathering logs from the **sources**, and storing in the temporary buffers.
* **Traffic Controller**: Makes the Loggregator Servers horizontally scalable by partitioning incoming log messages and outgoing traffic. Routes incoming log messages and proxies outgoing connections to the CLI and to drains for 3rd party partners.

Source agents emit the logging data as [protocol-buffers](https://code.google.com/p/protobuf/), and the data stays in that format throughout the system.

![Loggregator Diagram](docs/loggregator.png)

In a redundant CloudFoundry setup, Loggregator can be configured to survive zone failures. Log messages from non-affected zones will still make it to the end user. On AWS, availability zones could be used as redundancy zones. The following is an example of a multi zone setup with two zones.

![Loggregator Diagram](docs/loggregator_multizone.png)

The loggregator Traffic Controller has two roles.   

The first role is to take traffic from the various emitter sources (dea, dea-logging-agent router, etc) and route that traffic to one or more loggregator servers.   In the current config we route this traffic to the loggregator servers in the same az.   The traffic is sharded across loggregator servers by application id.    In this role the traffic between the traffic_controller and loggregator server(s) is all within the same az.

The second role is to handle inbound web socket requests for log data.    It does this by proxying the request to the correct loggregator server (sharded by application id) within all azs configured.  Since an application can be deployed to multiple az, it's logs can potentially end up in multiple azs.   This is why the traffic controller will attempt to connect to loggregator servers in each az and will collate the data into a single stream for the web socket client.    In the role the traffic between the traffic_controller and loggregator server(s) in across azs.

The traffic controller itself is stateless and a web socket request can be handle by any instance in any az.    The inbound emitter source connections could be load balanced across traffic controllers as well but we have yet to deploy this configuration.

### Emitting Messages from other Cloud Foundry components

Cloud Foundry developers can easily add source clients to new CF components that emit messages to the loggregator server.  Currently, there are libraries for [Go](https://github.com/cloudfoundry/loggregatorlib/tree/master/emitter) and [Ruby](https://github.com/cloudfoundry/loggregator_emitter). For usage information, look at their respective READMEs.

### Deploying via BOSH

Below are example snippets for deploying the DEA Logging Agent (source), Loggregator Server, and Loggregator Traffic Controller via BOSH.

```yaml
jobs:
- name: dea_next
  template:
  - dea_next
  - dea_logging_agent
  instances: 1
  resource_pool: dea
  networks:
  - name: cf1
    default:
    - dns
    - gateway

- name: loggregator
  template: loggregator
  instances: 1  # Scale out as neccessary
  resource_pool: common
  networks:
  - name: cf1
    static_ips:
    - 10.10.16.14

- name: loggregator_trafficcontroller
  template: loggregator_trafficcontroller
  instances: 1  # Scale out as necessary
  resource_pool: common
  networks:
  - name: cf1
    static_ips:
    - 10.10.16.16
  properties:
    traffic_controller:
      zone: z1 # Denoting which one of the redundancy zones this traffic controller is servicing

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

#### Capacity/Scaling

There is a the limitation about how many messages are retained for the --recent command. If you want to increase that number, you probably want to increase the number of Loggregator servers.

Increase the number of traffic controllers and Loggregator servers when to (better) handle many apps in a deployment. For each app we spin up a go routine (something like a java thread). There is a limit to how many you should spin up per go process.

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

#### OS X prequisites

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

#### Additional go tools

Install go vet and go cover

    go get code.google.com/p/go.tools/cmd/vet
    go get code.google.com/p/go.tools/cmd/cover

#### Running tests

```
bin/test
```

### Running specific tests

```
export GOPATH=`pwd` #in the root of the project
go get github.com/onsi/ginkgo/ginkgo
export PATH=$PATH:$GOPATH/bin
cd src/loggregator # or any other component
ginkgo -r
```

### Debugging


Loggregator will dump information about the running goroutines to stdout if sent a `USR1` signal.

```
goroutine 1 [running]:
runtime/pprof.writeGoroutineStacks(0xc2000bc3f0, 0xc200000008, 0xc200000001, 0xca0000c2001fcfc0)
	/home/travis/.gvm/gos/go1.1.1/src/pkg/runtime/pprof/pprof.go:511 +0x7a
runtime/pprof.writeGoroutine(0xc2000bc3f0, 0xc200000008, 0x2, 0xca74765c960d5c8f, 0x40bbf7, ...)
	/home/travis/.gvm/gos/go1.1.1/src/pkg/runtime/pprof/pprof.go:500 +0x3a
....
``` 
