package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

func main() {
	resultTag := flag.String("tag", "", "Tag the results with this prefix")
	dopplerCmd := flag.String("cmd", "", "The command to start doppler. (REQUIRED)")
	dopplerAddr := flag.String("doppler", "", "The address for fake doppler to bind to. (REQUIRED)")

	ca := flag.String("ca", "", "The path to the CA. (REQUIRED)")
	cert := flag.String("cert", "", "The path to the server cert. (REQUIRED)")
	key := flag.String("key", "", "The path to the server key. (REQUIRED)")
	cn := flag.String("cn", "doppler", "The TLS common name.")

	iterations := flag.Int("iter", 10000, "The number of envelopes to emit to doppler.")
	delay := flag.Duration("delay", 2*time.Microsecond, "The delay between envelope emission.")
	cycles := flag.Int("cycles", 5, "The number of tests to run")
	flag.Parse()

	if *dopplerCmd == "" {
		log.Fatal("Missing required flag 'cmd'")
	}

	if *dopplerAddr == "" {
		log.Fatal("Missing required flag 'doppler'")
	}

	if *ca == "" {
		log.Fatal("Missing required flag 'ca'")
	}

	if *cert == "" {
		log.Fatal("Missing required flag 'cert'")
	}

	if *key == "" {
		log.Fatal("Missing required flag 'key'")
	}

	if *cn == "" {
		log.Fatal("Missing required flag 'cn'")
	}

	creds, err := plumbing.NewClientCredentials(*cert, *key, *ca, *cn)
	if err != nil || creds == nil {
		log.Fatalf("Unable to setup TLS")
	}

	c := newConsumer()
	go c.Start(*dopplerAddr, creds)

	chunks := strings.SplitN(*dopplerCmd, " ", -1)

	cmd := exec.Command(chunks[0], chunks[1:]...)
	err = cmd.Start()
	if err != nil {
		log.Panicf("Failed to start doppler: %s", err)
	}
	defer cmd.Process.Kill()

	var results []int64
	for i := 0; i < *cycles; i++ {
		time.Sleep(5 * time.Second)
		atomic.StoreInt64(&c.count, 0)

		produceEnvelopes(*iterations, *delay, *dopplerAddr, creds)

		time.Sleep(5 * time.Second)

		log.Printf("Total Received: %d", atomic.LoadInt64(&c.count))
		results = append(results, atomic.LoadInt64(&c.count))
	}

	fmt.Printf("%s %v\n", *resultTag, results)
}

func produceEnvelopes(iterations int, delay time.Duration, dopplerAddr string, creds credentials.TransportCredentials) {
	conn, err := grpc.Dial(dopplerAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Panicf("Failed to dial gRPC server: %s", err)
	}
	defer conn.Close()
	client := plumbing.NewDopplerIngestorClient(conn)
	ctx := context.Background()
	pusher, err := client.Pusher(ctx)
	if err != nil {
		log.Panicf("Got error in establishing writer into doppler: %s", err)
	}

	for i := 0; i < iterations; i++ {
		env := &plumbing.EnvelopeData{
			Payload: createEnvelope(),
		}
		err := pusher.Send(env)
		if err != nil {
			log.Panicf("Failed to write data to doppler: %s", err)
		}

		time.Sleep(delay)
	}
}

func createEnvelope() []byte {
	e := &events.Envelope{
		Origin:    proto.String("doppler-benchmark"),
		EventType: events.Envelope_CounterEvent.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String(fmt.Sprintf("some-name-%d", rand.Intn(10))),
			Delta: proto.Uint64(uint64(rand.Intn(100))),
		},
	}

	data, err := proto.Marshal(e)
	if err != nil {
		log.Panic("Failed to marshal envelope: %s", err)
	}

	return data
}

type consumer struct {
	count int64
}

func newConsumer() *consumer {
	return &consumer{}
}

func (c *consumer) Start(addr string, creds credentials.TransportCredentials) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Panicf("Failed to dial gRPC server: %s", err)
	}
	defer conn.Close()
	client := plumbing.NewDopplerClient(conn)
	ctx := context.Background()
	req := &plumbing.SubscriptionRequest{
		ShardID: "doppler-benchmark",
	}
	subClient, err := client.Subscribe(ctx, req)
	if err != nil {
		log.Panicf("Got err in establishing reader from doppler: %s", err)
	}

	for {
		data, err := subClient.Recv()
		if err != nil {
			log.Printf("Failed while receiving: %s", err)
			continue
		}

		var e events.Envelope
		err = proto.Unmarshal(data.Payload, &e)
		if err != nil {
			log.Panicf("Failed to unmarshal received envelope: %s", err)
		}
		if e.GetOrigin() != "doppler-benchmark" {
			continue
		}

		atomic.AddInt64(&c.count, 1)
	}
}
