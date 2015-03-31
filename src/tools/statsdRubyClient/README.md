#Summary

This is a simple ruby client which can be used to send metrics to a statsd server running locally on port 8125.
The client reads the standard input or any file passed in as an argument for statsd commands. The
valid commands are:

```
timing <name> <value>
gauge <name> <value>
count <name> <value> [sample_rate]
```

# Running

Run `bundle install` to get dependencies and then run:

```
ruby statsdRubyClient.rb
```
