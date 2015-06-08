# Syslog Drain Binder

Syslog Drain Binder is a Cloud Foundry component that polls Cloud Controller for syslog drain bindings and updates etcd for [Doppler](../doppler). Doppler is notified of these changes and will forward logs and metrics to the new endpoint(s).

Each syslog drain binding belongs to a single app, but each app can have multiple syslog drain bindings.

Multiple instances of Syslog Drain Binder can and should be deployed, but only one is active at a given time (via an etcd [election process](elector/elector.go)).

## Usage
```syslog_drain_binder [--config <path to config file>] [--debug=<true|false>]```

| Flag           | Required                              | Description                                     |
|----------------|---------------------------------------|-------------------------------------------------|
| ```--config``` | No, default: ```config/syslog_drain_binder.json``` | Location of the Syslog Drain Binder configuration JSON file. |
| ```--debug```  | No, default: ```false```              | Debug logging                                   |

## Editing Manifest Templates
Currently the Syslog Drain Binder manifest configuration lives [here](../../manifest-templates/cf-lamb.yml). Editing this file will make changes in the [manifest templates](https://github.com/cloudfoundry/cf-release/tree/master/templates) in cf-release. When making changes to these templates, you should be working out of the loggregator submodule in cf-release. After changing this configuration, you will need to run the tests in root directory of cf-release with `bundle exec rspec`. These tests will pull values from [lamb-properties](../../manifest-templates/lamb-properties.rb) in order to populate the fixtures. Necessary changes should be made in [lamb-properties](../../manifest-templates/lamb-properties.rb).