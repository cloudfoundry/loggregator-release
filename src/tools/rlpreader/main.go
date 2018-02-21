// rlpreader: a tool that reads messages from RLP.
//
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing"
)

var (
	target            = flag.String("target", "localhost:8082", "the host:port of the target rlp")
	appID             = flag.String("app-id", "", "app-id to stream data")
	shardID           = flag.String("shard-id", "", "sets the shard_id field")
	deterministicName = flag.String("deterministic-name", "", "sets the deterministic_name field")
	certFile          = flag.String("cert", "", "cert to use to connect to rlp")
	keyFile           = flag.String("key", "", "key to use to connect to rlp")
	caFile            = flag.String("ca", "", "ca cert to use to connect to rlp")
	delay             = flag.Duration("delay", 0, "delay inbetween reading messages")
	preferredTags     = flag.Bool("preferred-tags", false, "use preferred tags")
	counterName       = flag.String("counter", "", "select a counter with the given name")
	gaugeNames        = flag.String("gauge", "", "select a gauge with the given comma separated names (must contain all the names)")

	metricNames = flag.Bool("metric-names", false, "this is really only useful while trying out deterministic routing. It only grabs counters and gauges and only prints their names")
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

	var selectors []*loggregator_v2.Selector
	if *counterName != "" {
		selectors = append(selectors, &loggregator_v2.Selector{
			SourceId: *appID,
			Message: &loggregator_v2.Selector_Counter{
				Counter: &loggregator_v2.CounterSelector{
					Name: *counterName,
				},
			},
		})
	}

	if *gaugeNames != "" {
		selectors = append(selectors, &loggregator_v2.Selector{
			SourceId: *appID,
			Message: &loggregator_v2.Selector_Gauge{
				Gauge: &loggregator_v2.GaugeSelector{
					Names: strings.Split(*gaugeNames, ","),
				},
			},
		})
	}

	if len(selectors) == 0 {
		selectors = append(selectors, &loggregator_v2.Selector{
			SourceId: *appID,
			Message: &loggregator_v2.Selector_Log{
				Log: &loggregator_v2.LogSelector{},
			},
		})
	}

	if *metricNames {
		selectors = []*loggregator_v2.Selector{
			{
				Message: &loggregator_v2.Selector_Counter{
					Counter: &loggregator_v2.CounterSelector{},
				},
			},
			{
				Message: &loggregator_v2.Selector_Gauge{
					Gauge: &loggregator_v2.GaugeSelector{},
				},
			},
		}
	}

	receiver, err := client.BatchedReceiver(context.TODO(), &loggregator_v2.EgressBatchRequest{
		ShardId:           buildShardID(*shardID),
		DeterministicName: *deterministicName,
		UsePreferredTags:  *preferredTags,
		Selectors:         selectors,
	})
	if err != nil {
		log.Fatal(err)
	}

	for {
		batch, err := receiver.Recv()
		if err != nil {
			log.Printf("stopping reader, got err: %s", err)
			return
		}
		for _, e := range batch.Batch {
			if *metricNames {
				if e.GetCounter() != nil {
					fmt.Printf("%s\n", e.GetCounter().GetName())
					continue
				}

				var names []string
				for name := range e.GetGauge().GetMetrics() {
					names = append(names, name)
				}

				fmt.Printf("%s\n", strings.Join(names, ", "))
				continue
			}

			fmt.Printf("%+v\n", e)
		}
		time.Sleep(*delay)
	}
}

func buildShardID(shardID string) string {
	if shardID == "" {
		return "rlp-reader-" + randString()
	}

	return shardID
}

func randString() string {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		log.Panicf("unable to read randomness %s:", err)
	}
	return fmt.Sprintf("%x", b)
}
