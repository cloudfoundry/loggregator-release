
# LATS - Loggregator Acceptance Tests

The loggregator acceptance tests are intended to be run against a CF
environment to test various loggregator components. The test suite is not part
of cf-release, but can be included by running the `bin/add_to_cf_release`
script. This test suite is intended to be run before loggregator is bumped in
cf-release.

The loggregator acceptance tests are intended to be run against a standalone
loggregator deployment. Use `scripts/generate-bosh-lite-manifest` to generate
a manifest.
