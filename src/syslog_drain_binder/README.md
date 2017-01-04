# Syslog Drain Binder

Syslog Drain Binder is a Cloud Foundry component that polls Cloud Controller for syslog drain bindings and updates etcd for [Doppler](../doppler). Doppler is notified of these changes and will forward logs and metrics to the new endpoint(s).

Each syslog drain binding belongs to a single app, but each app can have multiple syslog drain bindings.

Multiple instances of Syslog Drain Binder can and should be deployed, but only one is active at a given time (via an etcd [election process](elector/elector.go)).

## Usage
```syslog_drain_binder [--config <path to config file>]```

| Flag           | Required                              | Description                                     |
|----------------|---------------------------------------|-------------------------------------------------|
| ```--config``` | No, default: ```config/syslog_drain_binder.json``` | Location of the Syslog Drain Binder configuration JSON file. |
| ```--logFile```| No, default: STDOUT                   | The agent log file.                             |

## Editing Manifest Templates
The up-to-date Syslog Drain Binder configuration can be found [in the syslog_drain_binder spec file](../../jobs/syslog_drain_binder/spec). You can see a list of available configurable properties, their defaults and descriptions in that file.
