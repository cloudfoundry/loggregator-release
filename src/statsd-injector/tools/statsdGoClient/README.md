#Summary

This is a simple go client which can be used to send metrics to a statsd server running locally.
The client reads the standard input for statsd commands. The valid commands are:

```
timing <name> <value> [sample_rate]
gauge <name> <value>  [sample_rate]
count <name> <value>  [sample_rate]
```

# Running

Run:

```
go run main.go [PORT]
```

The optional parameter `PORT` specifies the port of the statsd server. It defaults to 8125.