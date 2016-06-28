# Bosh HM metrics Forwarder
## Setup with Bosh Lite

### Start local Metron Agent
1. `cd ~/workspace/lamb/src/github.com/cloudfoundry/loggregator/src/metron`
1. use router_z1 on the cf_warden deployment to get the metron_agent configuration
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
1. `cd ~/workspace/metrix-release/src/metrix/mars/boshhmforwarder`
1. create the /tmp/bosh-forwarder.json file
    ```
    {
      "port":4001,
      "metronPort": 3457
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
	}, {
   ```

   to find <your HostIP> use `ifconfig` and look for:

   ```
   en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500
    options=10b<RXCSUM,TXCSUM,VLAN_HWTAGGING,AV>
    ether ac:87:a3:2e:88:0a
    inet6 fe80::ae87:a3ff:fe2e:880a%en0 prefixlen 64 scopeid 0x4
    inet6 2001:559:803e:2:ae87:a3ff:fe2e:880a prefixlen 64 autoconf
    inet 10.35.33.57 netmask 0xffffff00 broadcast 10.35.33.255
    inet6 2001:559:803e:2:6580:79a:7a5d:5fa8 prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:1919:a6c9:fabb:4600 prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:419a:e9b7:203f:63e1 prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:d968:8500:3fa2:c759 prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:c10c:3f20:1804:886b prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:65a9:66d5:f1fe:408b prefixlen 64 deprecated autoconf temporary
    inet6 2001:559:803e:2:35f9:2678:f0aa:7c32 prefixlen 64 autoconf temporary
    nd6 options=1<PERFORMNUD>
    media: autoselect (1000baseT <full-duplex,flow-control>)
    status: active
   ```

   <your HostIP> is the inet entry above like 10.35.33.57
