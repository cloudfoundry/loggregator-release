# Firehose

## Filtering

The firehose supports filtering with a `filter-type` query param. The query
param may be set to `logs` to receive only log messsages, or it may be set to
`metrics` to receive only metrics.

For example, when requesting a subscription, the following request will return
only log messages.

```
http://loggregator.system-domain:8081/firehose/subscription-id?filter-type=logs
```

## Configuration

The firehose feature includes the combined stream of logs from all apps, plus
metrics data from CF components, and is intended to be used by operators and
administrators.

Access to the firehose requires a user with the `doppler.firehose` scope.

The "cf" UAA client needs permission to grant this custom scope to users.  The
configuration of the `uaa` job in Cloud Foundry
[adds this scope by default](https://github.com/cloudfoundry/cf-release/blob/2a3d95417da3c59564daeecd754eb00862030cd6/jobs/uaa/templates/uaa.yml.erb#L111).
However, if your Cloud Foundry instance overrides the
`properties.uaa.clients.cf` property in a stub, you need to add
`doppler.firehose` to the scope list in the `properties.uaa.clients.cf.scope`
property.

### Configuring at deployment time (via deployment manifest)

In your deployment manifest, add

```yaml
properties:
  # ...
  uaa:
    # ...
    clients:
      # ...
      cf:
        scope: …,doppler.firehose
      # ...
      doppler:
        override: true
        authorities: uaa.resource
        secret: YOUR-DOPPLER-SECRET
```

The `properties.uaa.clients.doppler.id` key should be populated
automatically. These are also set by default in
[cf-properties.yml](https://github.com/cloudfoundry/cf-release/blob/master/templates/cf-properties.yml#L304-L307).

### Adding scope to a running cluster (via `uaac`)

Before continuing, you should be familiar with the [`uaac`
tool](http://docs.cloudfoundry.org/adminguide/uaa-user-management.html).

Ensure that doppler is a UAA client. If `uaac client get doppler` returns
output like the following, then you're set.

```
scope: uaa.none
client_id: doppler
resource_ids: none
authorized_grant_types: authorization_code refresh_token
authorities: uaa.resource
```

If client does not exist, run the following and then set your secret.

```
uaac client add doppler --scope uaa.none \
  --authorized_grant_types authorization_code,refresh_token \
  --authorities uaa.resource
```

If the client already exists but with incorrect properties, run the following:

```
uaac client update doppler --scope uaa.none \
    --authorized_grant_types "authorization_code refresh_token" \
    --authorities uaa.resource
```

Then, grant firehose access to the `cf` client.

Next, check the scopes assigned to `cf` with `uaac client get cf`. The
resulting output will look something like the following:

```
  scope: cloud_controller.admin cloud_controller.read cloud_controller.write openid password.write scim.read scim.userids scim.write
  client_id: cf
  resource_ids: none
  authorized_grant_types: implicit password refresh_token
  access_token_validity: 600
  refresh_token_validity: 2592000
  authorities: uaa.none
  autoapprove: true
```

Finally, copy the existing scope and add `doppler.firehose`, then update the
client:

```
uaac client update cf \
    --scope "cloud_controller.admin cloud_controller.read cloud_controller.write openid password.write scim.read scim.userids scim.write doppler.firehose"
```
