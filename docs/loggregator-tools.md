Loggregator maintains several tools we find useful for testing the delivery of logs through Loggregator. 

### LogSpinner ###
Logspinner is a simple application written in golang that allows you to deliver a determined number of distinct log messages, on a given delay. It is available as part of the loggreagor repo in [`/loggregator/src/tools/logspinner`](https://github.com/cloudfoundry/loggregator/tree/develop/src/tools/logspinner). 

### Smoke Tests ###
Our smoke test is an automated deployment of the LogSpinner application and the delivery of 10,000 logs on a delay of 2 microseconds. The test then uses `cf logs` to count the number of results delivered through the firehose so that operators can determine loss in a worst case scenario (applications producing lots of logs). The test is configured to run in concourse every 15 minutes and is available in our [CI repo](https://github.com/cloudfoundry/loggregator-ci/blob/master/pipelines/smoke-tests.yml).
