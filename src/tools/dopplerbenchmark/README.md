# Doppler Benchmarking

Measure the performance of Doppler

Will measure messages per second, per minute, and per hour.

## How it Works

This is a command line tool that will emit X amount of messages over Y units of time.
* run etcd `etcd` (assuming it is installed)
* run doppler `bin/doppler` (from loggregator root, make sure it's the latest binary')
* run dopplerbenchmark `go run main.go` in loggregator/src/tools/dopplerbenchmark

## Command line options

| Flag | Default | Description |
|-------|--------|-------------|
| `-interval` | 1s | Interval for reported results |
| `-writeRate` | 15000 | Number of writes per second to send to doppler |
| `-stopAfter` | 5m | How long to run the experiment for |
| `-sharedSecret | "" | Shared secret used by Doppler to verify message validity |
