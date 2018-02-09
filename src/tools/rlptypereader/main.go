// rlpreader: a tool that reads messages from RLP.
//
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
)

var (
	target   = flag.String("target", "localhost:8082", "the host:port of the target rlp")
	certFile = flag.String("cert", "", "cert to use to connect to rlp")
	keyFile  = flag.String("key", "", "key to use to connect to rlp")
	caFile   = flag.String("ca", "", "ca cert to use to connect to rlp")

	shardID       = flag.String("shard-id", "a-shard-id", "shard ID for stream sharding")
	selectorTypes = flag.String("types", "", "comma separated list of envelope types to select on. default is all types.")
)

func main() {
	flag.Parse()

	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		*certFile,
		*keyFile,
		*caFile,
		"reverselogproxy",
	)
	if err != nil {
		log.Fatal(err)
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.Dial(*target, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		log.Fatal(err)
	}
	client := loggregator_v2.NewEgressClient(conn)

	req := &loggregator_v2.EgressRequest{
		ShardId:          *shardID,
		UsePreferredTags: true,
	}

	if *selectorTypes != "" {
		req.Selectors = buildSelectors(*selectorTypes)
	}

	receiver, err := client.Receiver(context.TODO(), req)
	if err != nil {
		log.Fatal(err)
	}

	for {
		env, err := receiver.Recv()
		if err != nil {
			log.Fatalf("failed to receive from stream: %s", err)
		}
		fmt.Printf("%+v\n", env)
	}
}

func buildSelectors(types string) []*loggregator_v2.Selector {
	chunks := strings.Split(types, ",")
	var selectors []*loggregator_v2.Selector
	for _, c := range chunks {
		selectors = append(selectors, stringToSelector(c))
	}

	return selectors
}

func stringToSelector(selectorType string) *loggregator_v2.Selector {
	switch strings.TrimSpace(strings.ToLower(selectorType)) {
	case "log":
		return &loggregator_v2.Selector{
			Message: &loggregator_v2.Selector_Log{
				Log: &loggregator_v2.LogSelector{},
			},
		}
	case "counter":
		return &loggregator_v2.Selector{
			Message: &loggregator_v2.Selector_Counter{
				Counter: &loggregator_v2.CounterSelector{},
			},
		}
	case "gauge":
		return &loggregator_v2.Selector{
			Message: &loggregator_v2.Selector_Gauge{
				Gauge: &loggregator_v2.GaugeSelector{},
			},
		}
	case "timer":
		return &loggregator_v2.Selector{
			Message: &loggregator_v2.Selector_Timer{
				Timer: &loggregator_v2.TimerSelector{},
			},
		}
	case "event":
		return &loggregator_v2.Selector{
			Message: &loggregator_v2.Selector_Event{
				Event: &loggregator_v2.EventSelector{},
			},
		}
	default:
		log.Fatalf("Unknown selector type: %s", selectorType)
	}
	return nil
}
