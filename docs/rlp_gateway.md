# Reverse Log Proxy (RLP) Gateway

The Reverse Log Proxy Gateway provides an HTTP API to access the Reverse Log
Proxy. The HTTP endpoint is only available on the local machine and is
unauthenticated.

## HTTP API

### GET /v2/read

Provides a stream to read envelope batches as Server Sent Events (SSE) in
JSON format.

#### Parameters

- `shard_id`  - Set the shard ID. Envelopes will be split between clients
  using the same source ID.
- `source_id` - One or more source IDs.
- `log`       - Request log envelopes.
- `counter`   - Request counter envelopes.
- `gauge`     - Request gauge envelopes.
- `timer`     - Request timer envelopes.
- `event`     - Request event envelopes.
- `counter.name` - Request counter envelopes and filter on counter name.
- `gauge.name`   - Request gauge envelopes and filter on gauge name.
- `deterministic_name` - Enable deterministic routing.

A 400 Bad Request is returned when no envelope types are passed into the query
string.

#### Example Requests

Request log envelopes
```
curl http://localhost:8088/v2/read?log
```

Request log envelopes with sharding:
```
curl http://localhost:8088/v2/read?log&shard_id=shard-id-for-all-clients
```

Request all envelopes for a given source:
```
curl http://localhost:8088/v2/read?log&counter&gauge&timer&event&source_id=SOURCE-ID
```

Request counter metrics with a given name:
```
curl http://localhost:8088/v2/read?counter.name=request_count
```

#### Example SSE Response

```
data: {"batch":[{"timestamp":"1532030745755909241","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"gauge":{"metrics":{"subscriptions":{"unit":"subscriptions","value":1}}}}]}

data: {"batch":[{"timestamp":"1532030745755669593","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"egress","delta":"9","total":"1462"}},{"timestamp":"1532030745755852038","sourceId":"doppler","tags":{"deployment":"loggregator","direction":"ingress","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"dropped"}},{"timestamp":"1532030745756105677","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"gauge":{"metrics":{"dump_sinks":{"unit":"sinks","value":1}}}},{"timestamp":"1532030745755954729","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"ingress","delta":"9","total":"20260"}},{"timestamp":"1532030745755989588","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"egress","total":"1462"}},{"timestamp":"1532030745756023906","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"sinks.errors.dropped"}},{"timestamp":"1532030745756066632","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"counter":{"name":"sinks.dropped"}},{"timestamp":"1532030745755810461","sourceId":"doppler","tags":{"deployment":"loggregator","index":"43a85aeb-89d1-4d36-9258-d781b571fe32","ip":"10.244.0.128","job":"doppler","metric_version":"2.0","origin":"loggregator.doppler"},"gauge":{"metrics":{"container_metric_sinks":{"unit":"sinks","value":1}}}}]}
```
