
### Configuring the Firehose

The firehose feature includes the combined stream of logs from all apps, plus metrics data from CF components, and is intended to be used by operators and administrators.

Access to the firehose requires a user with the `doppler.firehose` scope.

The "cf" UAA client needs permission to grant this custom scope to users.
The configuration of the `uaa` job in Cloud Foundry [adds this scope by default](https://github.com/cloudfoundry/cf-release/blob/2a3d95417da3c59564daeecd754eb00862030cd6/jobs/uaa/templates/uaa.yml.erb#L111).
However, if your Cloud Foundry instance overrides the `properties.uaa.clients.cf` property in a stub, you need to add `doppler.firehose` to the scope list in the `properties.uaa.clients.cf.scope` property.

#### Configuring at deployment time (via deployment manifest)

In your deployment manifest, add

```yaml
properties:
  …
  uaa:
    …
    clients:
      …
      cf:
        scope: …,doppler.firehose
      …
      doppler:
        override: true
        authorities: uaa.resource
        secret: YOUR-DOPPLER-SECRET
```

(The `properties.uaa.clients.doppler.id` key should be populated automatically.) These are also set by default in [cf-properties.yml](https://github.com/cloudfoundry/cf-release/blob/master/templates/cf-properties.yml#L304-L307).

#### Adding scope to a running cluster (via `uaac`)

Before continuing, you should be familiar with the [`uaac` tool](http://docs.cloudfoundry.org/adminguide/uaa-user-management.html).

1. Ensure that doppler is a UAA client. If `uaac client get doppler` returns output like

  ```
    scope: uaa.none
    client_id: doppler
    resource_ids: none
    authorized_grant_types: authorization_code refresh_token
    authorities: uaa.resource
  ```

  then you're set.

  1. If it does not exist, run `uaac client add doppler --scope uaa.none --authorized_grant_types authorization_code,refresh_token --authorities uaa.resource` (and set its secret).
  1. If it exists but with incorrect properties, run `uaac client update doppler --scope uaa.none --authorized_grant_types "authorization_code refresh_token" --authorities uaa.resource`.
1. Grant firehose access to the `cf` client.
  1. Check the scopes assigned to `cf` with `uaac client get cf`, e.g.

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

  1. Copy the existing scope and add `doppler.firehose`, then update the client

    ```
    uaac client update cf --scope "cloud_controller.admin cloud_controller.read cloud_controller.write openid password.write scim.read scim.userids scim.write doppler.firehose"
    ```
