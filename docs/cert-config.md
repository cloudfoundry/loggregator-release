
### Generating TLS Certificates

To generate the Loggregator TLS certs and keys, run
scripts/generate-loggregator-certs <diego-bbs-ca.crt> <diego-bbs-ca.key>. The
diego BBS CA cert and key are typically generated separately from this script.

#### Using bosh 2.0

If you are using the [new bosh cli](https://github.com/cloudfoundry/bosh-cli) you can generate certs using `--vars-store` flag.

```
bosh -e lite -d loggregator deploy templates/loggregator.yml
--vars-store=loggregator-vars.yml
```

#### Custom TLS Certificates

If you already have a CA, you'll need to create a certificate for each component.  Values for the components are:

##### Doppler
- common name: doppler
- extended key usage: serverAuth, clientAuth

##### TrafficController
- common name: trafficcontroller
- extended key usage: serverAuth, clientAuth

##### Metron
- common name: metron
- extended key usage: serverAuth, clientAuth

### Adding your TLS certificates

#### Doppler

| Property                       | Required | Description                                         |
|--------------------------------|----------|-----------------------------------------------------|
| `loggregator.tls.doppler.cert` | Yes      | Certificate used by doppler to communicate over TLS |
| `loggregator.tls.doppler.key`  | Yes      | Key used by doppler to communicate over TLS         |
| `loggregator.tls.ca_cert`      | Yes      | Certificate Authority used to sign the certificate  |

#### Traffic Controller

| Property                                 | Required | Description                                                    |
|------------------------------------------|----------|----------------------------------------------------------------|
| `loggregator.tls.trafficcontroller.cert` | Yes      | Certificate used by traffic controller to communicate over TLS |
| `loggregator.tls.trafficcontroller.key`  | Yes      | Key used by traffic controller to communicate over TLS         |
| `loggregator.tls.ca_cert`                | Yes      | Certificate Authority used to sign the certificate             |

#### Metron

| Property                      | Required | Description                                        |
|-------------------------------|----------|----------------------------------------------------|
| `loggregator.tls.metron.cert` | Yes      | Certificate used by metron to communicate over TLS |
| `loggregator.tls.metron.key`  | Yes      | Key used by metron to communicate over TLS         |
| `loggregator.tls.ca_cert`     | Yes      | Certificate Authority used to sign the certificate |

#### Example Manifest

```yaml
  loggregator:
    tls:
      ca_cert: |
        -----BEGIN CERTIFICATE-----
        LOGGREGATOR CA CERTIFICATE
        -----END CERTIFICATE-----
      doppler:
        cert: |
          -----BEGIN CERTIFICATE-----
          DOPPLER CERTIFICATE
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          DOPPLER KEY
          -----END RSA PRIVATE KEY-----
      trafficcontroller:
        cert: |
          -----BEGIN CERTIFICATE-----
          TRAFFIC CONTROLLER CERTIFICATE
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          TRAFFIC CONTROLLER KEY
          -----END RSA PRIVATE KEY-----
      metron:
        cert: |
          -----BEGIN CERTIFICATE-----
          METRON CERTIFICATE
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          METRON KEY
          -----END RSA PRIVATE KEY-----
```

### Enabling TLS between Loggregator and etcd

By default, doppler, syslog_drain_binder, and loggregator_trafficcontroller all communicate with etcd over
http.  To enable TLS mutual auth to etcd, you'll need to generate certificates and update your manifest

#### etcd

Refer to [etcd-release's guide on Encrypting Traffic](https://github.com/cloudfoundry-incubator/etcd-release#encryption) for etcd's properties.

#### Doppler

| Property        | Required                              | Description                                     |
|-----------------|---------------------------------------|-------------------------------------------------|
| `loggregator.etcd.require_ssl` | No<br> Default: `false`                   | Enable ssl for all communcation with etcd |
| `loggregator.etcd.ca_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded CA certificate            |
| `doppler.etcd.client_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client certificate            |
| `doppler.etcd.client_key`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client key            |

#### Traffic Controller

| Property        | Required                              | Description                                     |
|-----------------|---------------------------------------|-------------------------------------------------|
| `loggregator.etcd.require_ssl` | No<br> Default: `false`                   | Enable ssl for all communcation with etcd |
| `loggregator.etcd.ca_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded CA certificate            |
| `traffic_controller.etcd.client_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client certificate            |
| `traffic_controller.etcd.client_key`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client key            |

#### Syslog Drain Binder

| Property        | Required                              | Description                                     |
|-----------------|---------------------------------------|-------------------------------------------------|
| `loggregator.etcd.require_ssl` | No<br> Default: `false`                   | Enable ssl for all communcation with etcd |
| `loggregator.etcd.ca_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded CA certificate            |
| `syslog_drain_binder.etcd.client_cert`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client certificate            |
| `syslog_drain_binder.etcd.client_key`   | Yes if `loggregator.etcd.require_ssl` is set to `true` <br>Default: `""`              | PEM-encoded client key            |

An example manifest is given below:

```yaml
  loggregator:
    etcd:
      require_ssl: true
      ca_cert: |
        -----BEGIN CERTIFICATE-----
        ETCD CA CERTIFICATE
        -----END CERTIFICATE-----


  doppler:
    etcd:
      client_cert: |
        -----BEGIN CERTIFICATE-----
        DOPPLER CERTIFICATE
        -----END CERTIFICATE-----
      client_key: |
        -----BEGIN RSA PRIVATE KEY-----
        DOPPLER KEY
        -----END RSA PRIVATE KEY-----

  traffic_controller:
    etcd:
      client_cert: |
        -----BEGIN CERTIFICATE-----
        TRAFFIC CONTROLLER CERTIFICATE
        -----END CERTIFICATE-----
      client_key: |
        -----BEGIN RSA PRIVATE KEY-----
        TRAFFIC CONTROLLER KEY
        -----END RSA PRIVATE KEY-----

  syslog_drain_binder:
    etcd:
      client_cert: |
        -----BEGIN CERTIFICATE-----
        SYSLOG DRAIN BINDER CERTIFICATE
        -----END CERTIFICATE-----
      client_key: |
        -----BEGIN RSA PRIVATE KEY-----
        SYSLOG DRAIN BINDER KEY
        -----END RSA PRIVATE KEY-----
```

### Deploying via BOSH

Below are example snippets for deploying the DEA Logging Agent (source), Doppler, and Loggregator Traffic Controller via BOSH.

```yaml
jobs:
- name: dea_next
  templates:
  - name: dea_next
    release: cf
  - name: dea_logging_agent
    release: cf
  - name: metron_agent
    release: cf
  instances: 1
  resource_pool: dea
  networks:
  - name: cf1
    default:
    - dns
    - gateway
  properties:
    dea_next:
      zone: z1
    metron_agent:
      zone: z1
    networks:
      apps: cf1

- name: doppler_z1 # Add "doppler_zX" jobs if you have runners in zX
  templates:
  - name: doppler
    release: cf
  - name: syslog_drain_binder
    release: cf
  - name: metron_agent
    release: cf
  instances: 1  # Scale out as neccessary
  resource_pool: common
  networks:
  - name: cf1
  properties:
    doppler:
      zone: z1
    networks:
      apps: cf1

- name: loggregator_trafficcontroller_z1
  templates:
  - name: loggregator_trafficcontroller
    release: cf
  - name: metron_agent
    release: cf
  instances: 1  # Scale out as necessary
  resource_pool: common
  networks:
  - name: cf1
  properties:
    traffic_controller:
      zone: z1 # Denoting which one of the redundancy zones this traffic controller is servicing
    metron_agent:
      zone: z1
    networks:
      apps: cf1

properties:
  loggregator:
    servers:
      z1: # A list of loggregator servers for every redundancy zone
      - 10.10.16.14
    incoming_port: 3456
    outgoing_port: 8080

  loggregator_endpoint: # The end point sources will connect to
    shared_secret: loggregatorEndPointSharedSecret
    host: 10.10.16.16
    port: 3456
```
