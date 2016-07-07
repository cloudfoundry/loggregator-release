# Bosh HM metrics Forwarder

## Purpose

The purpose of the Bosh HM Forwarder is to forward bosh health metrics into
Loggregator thus making them available in the firehose.

Bosh Health Metrics are currently emitted as `bosh.healthmonitor.system.*`

## Architecture

Currently the Bosh Director is capable of emitting metrics via its [plugin
architecture](http://bosh.io/docs/monitoring.html). The Bosh HM Forwarder acts
as an OpenTSDB listener which reads off the metrics forwarded by the
Bosh Health Monitor.

The Bosh HM Forwarder must be colocated with a fully functioning Metron Agent.
It then forwards the metrics to Dopplers via the Metron Agent as ValueMetric
type.

## Setup with Bosh Lite

### Start local Metron Agent
1. `cd ~/workspace/loggregator/src/metron`
1. Use router_z1 on the cf_warden deployment to get the metron_agent configuration
   copy that information to /tmp/metron_config.json
   For Example:

  ```
  {
    "Index": 0,
    "Job": "router_z1",
    "Zone": "z1",
    "Deployment": "cf-warden",

    "EtcdUrls": ["http://10.244.0.42:4001"],
    "EtcdMaxConcurrentRequests": 10,

    "SharedSecret": "loggregator-secret",

    "LegacyIncomingMessagesPort": 3456,
    "DropsondeIncomingMessagesPort": 3457,

    "EtcdQueryIntervalMilliseconds": 5000,

    "LoggregatorDropsondePort": 3457,
    "Syslog": "vcap.metron_agent"
  }
  ```

1. `go run main.go -config /tmp/metron_config.json`

### Start local Bosh-Hm-Forwarder
1. `cd ~/workspace/loggregator/src/boshhmforwarder`
1. Create the /tmp/bosh-forwarder.json file
    ```
    {
      "IncomingPort":4001,
      "MetronPort": 3457
    }
    ```
1. `go run main.go --configPath /tmp/bosh-forwarder.json`

### Enable Bosh Health Monitor
1. `cd ~/workspace/bosh-lite`
1. `vagrant ssh`
1. `sudo -i`
1. `vi /var/vcap/jobs/health_monitor/config/health_monitor.yml`
1. add the following under 'plugins:' (line 32ish)
   ```
   plugins:
   [...]
     - name: tsdb
       events:
         - alert
         - heartbeat
       options:
         host: <your HostIP>
         port: 4001
   ```

   or (in case of JSON config)

   ```
   "plugins": [{
		"name": "tsdb",
		"events": ["alert", "heartbeat"],
		"options": {
			"host": "10.244.5.35",
			"port": 4000
		}
	}, {
		"name": "logger",
		"events": ["alert"]
	}, { ... }
	]
   ```

   to find <your HostIP> use `ifconfig` and look for:

   ```
   en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
    ...
    inet 10.35.33.57 netmask 0xffffff00 broadcast 10.35.33.255
    ...
    status: active
   ```

   <your HostIP> is the inet entry above like 10.35.33.57
1. `monit restart health_monitor`

## Setup With Loggregator as a Separate Release

1. Add the loggregator release to the manifest

  ```yaml
  - name: loggregator
    release: latest
  ```

1. Add the boshhmforwarder to the instance group/job

  ```yaml
  - name: boshhmforwarder
    release: loggregator
  ```

  NOTE: Ideally, the Bosh HM Forwarder should be located on a **single** instance
  that has a Metron Agent on it. As such, it can be deployed on a separate vm
  with the Metron Agent colocated on it.
