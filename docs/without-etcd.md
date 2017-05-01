# Running Loggregator without etcd

Etcd is no longer required to deploy loggregator. To achieve an etcd-less
deployment you must do the following:

 - Use bosh links for Traffic Controller's discovery of Dopplers.
 - Set `loggregator.disable_syslog_drains` to `true` for both Dopplers and
   Syslog Drain Binders.
 - Set `doppler.disable_announce` to `true` for Dopplers.
 - Set `instances` for etcd to `0` or remove the instance group entirely.

If you are deploying loggregator stand alone you can use the manifest under
`templates/loggregator.yml`.

If you are using `cf-deployment` you can use the ops file under
`operations/experimental/disable-etcd.yml`
