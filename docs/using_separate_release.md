# Summary

You can emit logs and metrics into the Loggregator subsystem from a service or other BOSH deployment that is separate from the CF deployment. This allows your logs and metrics to be available via the same firehose as all of the CF logs and metrics. You add the Metron agent (point of entry to the Loggregator subsystem) to your deployment and configure it to point at the Dopplers in your CF deployment. Then you send dropsonde messages (library support for some languages, see below) to Metron.

#Prerequisites

You need a CF deployment. From that deployment you need the following:
- ETCD IPs to use to retrieve doppler IPs

A service being deployed by bosh. You will modify the deployment manifest to pull in the loggregator bosh release, 
and then deploy the Metron agent from that release in your service deployment. 

#Steps

- Get the latest loggregator release from bosh.io
- Make manifest changes in your deployment to include the loggregator release and get the metron_agent job from loggregator

Manifest changes:
- Add the loggregator release into the manifest
```diff
+- name: loggregator
+  version: latest
```

- For **each job** that will emit logs or metrics, embed the metron_agent section into the manifest

```diff
+  - name: metron_agent
+    release: loggregator
```

- Insert the overall metron properties into the manifest. 
 - Note that setting ```prefered_protocol``` to ```null``` will cause the transport layer to use UDP. 
 This is not recommended because of the possibility for log and metric loss. It is recommended to 
 set ```preferred_protocol``` to TLS, adding appropriate certificates, using the instructions found [here](fill in)
 
```diff
properties:
+  metron_agent:
+    zone: z1
+    deployment: redis
+    preferred_protocol: null
+    shared_secret: "loggregator-secret"
+    tls_client:
+      cert: null
+      key: null
+  loggregator:
+    etcd:
+      machines:
+      - 10.244.0.42
```

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

