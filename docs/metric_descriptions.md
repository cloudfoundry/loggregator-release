### Summary
Loggregator emits a variety of metrics.

## Metron Egress
Monitoring metron engress can be be helpful for identifying the
top producers of logs and metrics on your platform. This can be accomplished
by sorting the top results for the max metric `metron.egress` sort by bosh job name.

Example datadog chart json:
```
{
  "viz": "toplist",
  "status": "done",
  "requests": [
    {
      "q": "top(avg:datadog.nozzle.loggregator.metron.egress{*} by {job}, 10, 'mean', 'desc')",
      "style": {
        "palette": "dog_classic"
      },
      "conditional_formats": []
    }
  ],
  "autoscale": true
}
```

Additionally, monitoring for rate of egress can ensure no single metron exceeds
the capacity of a Doppler, and that the sume does not exceed your Doppler capacity.
For more on Doppler scaling see the [Loggregator Operator Guidebook](./Loggregator%20Operator%20Guidebook.pdf).

Example datadog chart json:
```
{
  "viz": "timeseries",
  "status": "done",
  "requests": [
    {
      "q": "per_second(sum:datadog.nozzle.loggregator.metron.egress{*} by {job})",
      "style": {
        "palette": "dog_classic"
      },
      "conditional_formats": [],
      "type": "area",
      "aggregator": "avg"
    }
  ],
  "autoscale": true
}
```

## Doppler Ingress and Drops

Monitoring doppler ingress is an effective way to manage the number of dopplers in your deployment.
For more on Doppler scaling see the [Loggregator Operator Guidebook](./Loggregator%20Operator%20Guidebook.pdf).

Example datadog chart json:
```
{
  "viz": "query_value",
  "status": "done",
  "requests": [
    {
      "q": "per_second(sum:datadog.nozzle.loggregator.doppler.ingress{deployment:cf-cfapps-io2}) + per_second(sum:datadog.nozzle.DopplerServer.listeners.receivedEnvelopes{deployment:cf-cfapps-io2})",
      "aggregator": "max",
      "conditional_formats": [],
      "type": "area"
    }
  ],
  "autoscale": true
}
```

Usingthe metric formula `doppler.dropped / doppler.ingress` to calculate a drop percentage is an effective way
of monitoring for the handling of peak loads.

Example datadog chart json:

```
{
  "viz": "timeseries",
  "requests": [
    {
      "q": "( per_second(sum:datadog.nozzle.loggregator.doppler.dropped{*}) / per_second(sum:datadog.nozzle.loggregator.doppler.ingress{*}) ) * 100",
      "aggregator": "avg",
      "conditional_formats": [],
      "type": "line",
      "style": {
        "palette": "dog_classic"
      }
    }
  ],
  "autoscale": true,
  "status": "done",
  "markers": [
    {
      "val": 1,
      "value": "y = 1",
      "type": "error dashed",
      "dim": "y"
    }
  ]
}
```

## Traffic Controller Slow Consumers
Slow consumers indicate that there are subscribers to the Loggregator system that can not
keep up with the production of logs, and have been cut off. This could result from a
developer running `cf logs` on a bad connection or could be a Firehose Nozzle that can not keep up.
Monitoring slow consumers can be an important tool for troubleshooting overall system loss and
managing Nozzle and Traffic Controller scaling.

For more on Traffic Controler scaling see the [Loggregator Operator Guidebook](./Loggregator%20Operator%20Guidebook.pdf).

Example datadog json:
```
{
  "requests": [
    {
      "q": "piecewise_constant(per_hour(avg:datadog.nozzle.loggregator.trafficcontroller.doppler_proxy.slow_consumer{*}))",
      "type": "line",
      "conditional_formats": [],
      "style": {
        "type": "dashed"
      },
      "aggregator": "avg"
    },
    {
      "q": "per_hour(avg:datadog.nozzle.loggregator.trafficcontroller.doppler_proxy.slow_consumer{*})",
      "type": "line",
      "conditional_formats": []
    }
  ],
  "viz": "timeseries",
  "autoscale": true,
  "status": "done"
}
```

## Syslog Drain Metrics
There are metrics relevant to syslog drains that are documented in the [cf-syslog-drain repo](https://github.com/cloudfoundry/cf-syslog-drain-release#operator-metrics).



### Searching for Metrics in Loggregator
As of Loggregator 82 Metrics are now documented inline with specifics about what the metric represents. Here are some searches that you can use to get a summary of all metrics.

[`metrics-documentation-v1`](https://github.com/cloudfoundry/loggregator-release/search?utf8=%E2%9C%93&q=metric-documentation-v1&type=Code) - These are all metrics related to Dropsonde envelopes.

[`metrics-documentation-v2`](https://github.com/cloudfoundry/loggregator-release/search?utf8=%E2%9C%93&q=metric-documentation-v2&type=Code) - These are all metrics for the new Loggretor API V2

[`DEPRECATED`](https://github.com/cloudfoundry/loggregator-release/search?utf8=%E2%9C%93&q=DEPRECATED) - These are all metrics that will be deprecated in a future release. They are usually adjacent to a new replacement for the metric or no longer provide value.

[`USELESS`](https://github.com/cloudfoundry/loggregator-release/search?utf8=%E2%9C%93&q=USELESS) - These metrics do not provide a meaningful datapoint and should not be used.


