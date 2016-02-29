# Metron Benchmarking

Measure the performance of Metron

Will measure messages per second, per minute, and per hour.

## How it Works

This is a command line tool that will emit X amount of messages over Y units of time.
* run etcd `etcd` (assuming it is installed)
* run metron `bin/metron` (from loggregator root, make sure it's the latest binary')
* run metronbenchmark `go run main.go` in loggregator/src/tools/metronbenchmark

## Command line options

|         Flag         |       Default                                       |             Description                                 |
|----------------------|-----------------------------------------------------|---------------------------------------------------------|
| `-interval`          |    1s                                               | Interval for reported results                           |
| `-writeRate`         | 15000                                               | Number of writes per second to send to metron           |
| `-stopAfter`         |    5m                                               | How long to run the experiment for                      |
| `-eventType`         | ValueMetric                                         | The event type to use for the experiment                |
| `-concurrentWriters` |     1                                               | The number of writers to run concurrently               |
| `protocol`           |   udp                                               | The output protocol to benchmark                        |
| `serverCert`         | ../../integration_tests/fixtures/server.crt         | The certificate to serve TLS connections to metron with |
| `serverKey`          | ../../integration_tests/fixtures/server.key         | The key to serve TLS connections to metron with         |
| `caCert`             | ../../integration_tests/fixtures/loggregator-ca.crt | The CA cert to serve TLS connections to metron with     |


