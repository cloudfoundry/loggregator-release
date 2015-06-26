# Metron Benchmarking

Measure the performance of Metron

Will measure messages per second, per minute, and per hour.

## How it Works

This is a command line tool that will emit X amount of messages over Y units of time.
* run etcd `etcd` (assuming it is installed)
* run metron `bin/metron` (from loggregator root, make sure it's the latest binary')
* run metronbenchmark `go run main.go` in loggregator/src/tools/metronbenchmark