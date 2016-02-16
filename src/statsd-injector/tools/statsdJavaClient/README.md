#Summary

This is a simple java client which can be used to send metrics to a statsd server running locally on port 8125.
The client reads the standard input for statsd commands. The valid commands are:

```
timing <name> <value>
gauge <name> <value>
count <name> <value> [sample_rate]
```

# Running

Install gradle and run `gradle build` to get dependencies and then run:

```
java -jar build/libs/statsdJavaClient-0.0.1.jar
```
