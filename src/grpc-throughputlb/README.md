gRPC Throughput Load Balancer
===============================================================================

The gRPC throughput load balancer is a load balancer that implements the
[`grpc.Balancer
interface`](https://godoc.org/google.golang.org/grpc#Balancer). It will open a
configured number of connections to a single address and not allow more than a
given number of concurrent requests per address.

In your code when you make a gRPC request (stream or RPC), the gRPC throughput
load balancer will return the connection with the least number of active
requests.

## Load Balancer Lifecycle

The order gRPC calls methods on the gRPC throughput load balancer are:
1. Start
1. Notify
1. Up
1. Get

## Example

``` go
lb := throughputlb.NewThroughputLoadBalancer(100, 20)

conn, err := grpc.Dial(os.Getenv("GRPC_ADDR"),
    grpc.WithBalancer(lb))
if err != nil {
    panic(err)
}
```

