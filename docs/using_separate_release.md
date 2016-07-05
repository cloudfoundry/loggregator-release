# Summary

You can emit logs and metrics into the Loggregator subsystem from a service or other BOSH deployment that is separate from the CF deployment. This allows your logs and metrics to be available via the same firehose as all of the CF logs and metrics. You add the Metron agent (point of entry to the Loggregator subsystem) to your deployment and configure it to point at the Dopplers in your CF deployment. Then you send dropsonde messages (library support for some languages, see below) to Metron.

## Prerequisites

You need a CF deployment. From that deployment you need the following:
- ETCD IPs to use to retrieve Doppler addresses

You need a service being deployed by bosh. You will modify the deployment manifest to pull in the Loggregator bosh release,
and then deploy the Metron agent from that release in your service deployment.

## Steps to Deploy Metron Agent

- Get the latest [loggregator release from bosh.io](http://bosh.io/releases/github.com/cloudfoundry/loggregator)
- Make manifest changes in your deployment to include the loggregator release and get the metron_agent job from Loggregator

Manifest changes:
- Add the loggregator release into the manifest
```yaml
 - name: loggregator
   version: latest
```

- For **each job** that will emit logs or metrics, embed the metron_agent section into the manifest

```yaml
 - name: metron_agent
   release: loggregator
```

- Insert the overall metron properties into the manifest.
 - Note that setting ```protocols``` to ```null``` will cause the transport layer to use UDP.
 This is not recommended because of the possibility for log and metric loss. It is recommended to
 set ```protocols``` to TLS, adding appropriate certificates. Follow [these instructions](https://github.com/cloudfoundry/loggregator#enabling-tls-between-metron-and-doppler).

```yaml
properties:
 metron_agent:
   zone: z1
   deployment: redis
   preferred_protocol: null
   shared_secret: "loggregator-secret"
   tls_client:
     cert: null
     key: null
 loggregator:
   etcd:
     machines:
     - 10.244.0.42

```

## CF etcd in TLS mode
If CF's etcd cluster is running in TLS mode, the following properties need to be added to the manifest in order for the Metron Agent to communicate with the CF etcd. The properties above should ideally be pulled in from the CF manifest.

**NOTE: TLS support is currently experimental. Enable it at your own discretion. The properties discussed below as well as their behavior might change in the future.**

```yaml
properties:
 metron_agent:
   etcd:
     client_cert: |
        -----BEGIN CERTIFICATE-----
        METRON AGENT CERTIFICATE
        -----END CERTIFICATE-----
     client_key: |
        -----BEGIN RSA PRIVATE KEY-----
        METRON AGENT KEY
        -----END RSA PRIVATE KEY-----
 loggregator:
   etcd:
     require_ssl: true
     ca_cert: |
      -----BEGIN CERTIFICATE-----
      ETCD CA CERTIFICATE
      -----END CERTIFICATE----
```

## Use

- Modify your service app to create metrics or logging messages by using the dropsonde protocol.
 - Golang: use the dropsonde library. See the examples [here](https://github.com/cloudfoundry/dropsonde).
 - Ruby: use [loggregator_emitter](https://github.com/cloudfoundry/loggregator_emitter) to emit log messages. It
 currently does not support metrics. The Loggregator team is working on this.
 - Other languages
  - The current strategy is to use protobuf to generate the source files from dropsonde-protocol for your language.
  Serialize your protobuf messages and send them to metron on port 3457 over UDP. Note that the Loggregator roadmap
  includes changing to the client->Metron interface to TCP. This is a breaking change for an implementation based
  directly on protobuf. The Loggregator team encourages anyone who wants to use a different language to write an
  abstraction library, similar to dropsonde, and create a github repository. Please feel free to connect with the loggregator
  team via cf-loggregator@pivotal.io about this. If appropriate we would like to add language-specific dropsonde-protocol implementations
  to our CI tests.

## Steps to Deploy Bosh HM Forwarder

Please see instructions [here](https://github.com/cloudfoundry/loggregator/tree/develop/src/boshhmforwarder#setup-with-loggregator-as-a-separate-release)

## Backward and Forward Compatibility

Deploying Metron into a service, separate from a Cloud Foundry deploy, means that you will have a Metron of one version talking to a Doppler of another version. This means that changes in the dropsonde protocol as the Loggregator system evolves could result in different versions of the protocol being used by communicating Metrons and Dopplers. In order to allow developer and operators to understand the possible ramifications of a difference in protocol, here is the behavior "contract" between Doppler and Metron:

### Situation 1: Metron uses an older version of dropsonde than Doppler and there are differences

- Doppler implements a new field of functionality that Metron does not support
 - Metron has a missed opportunity to use a newer feature, but nothing breaks.
- Metron emits a field that Doppler no longer supports
 - Doppler drops the dropsonde message, and logs an error message

### Situation 2: Metron uses a newer version of dropsonde than Doppler and there are differences

- Metron emits a field that Doppler does not yet support
 - Doppler drops the dropsonde message, and logs an error message

