# statsd-injector
Companion component to Metron that receives Statsd and emits Dropsonde to Metron

## Getting started

The following instructions may help you get started with statsd-injector in a
standalone environment.

### External Dependencies

- Go should be installed and in the PATH
- [etcd](https://github.com/coreos/etcd) should be installed and in the PATH
- GOPATH should be set as described in http://golang.org/doc/code.html
- [loggregator](https://github.com/cloudfoundry/loggregator) should be included in the GOPATH if you want to run integration tests.

```
export GOPATH=$GOPATH:$WORKSPACE/loggregator
```

## Running Tests

We are using [Ginkgo](https://github.com/onsi/ginkgo), to run tests. To run the tests execute:

```bash
bin/test
```

## Including statsd-injector in a bosh deployment
As an example, if you want the injector to be present on loggregator boxes, add the following in `cf-lamb.yml`

```diff
   loggregator_templates:
   - name: doppler
     release: (( lamb_meta.release.name ))
   - name: syslog_drain_binder
     release: (( lamb_meta.release.name ))
   - name: metron_agent
     release: (( lamb_meta.release.name ))
+  - name: statsd-injector
+    release: (( lamb_meta.release.name ))
```

## Emitting metrics to the statsd-injector
You can emit statsd metrics to the injector by sending a correctly formatted message to udp port 8125

As an example using netcat:

```
echo "origin.some.counter:1|c" | nc -u -w0 127.0.0.1 8125
```

You should see the metric come out of the firehose.

The injector expects the the name of the metric to be of the form `<origin>.<metric_name>`
