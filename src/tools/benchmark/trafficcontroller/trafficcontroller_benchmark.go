package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	config "code.cloudfoundry.org/loggregator/doppler/app"
	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"

	"code.cloudfoundry.org/loggregator/dopplerservice"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/workpool"

	noaa "github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/gogo/protobuf/proto"
)

func main() {
	resultTag := flag.String("tag", "", "Tag the results with this prefix")
	trafficcontrollerCmd := flag.String("cmd", "", "The command to start trafficcontroller. (REQUIRED)")
	tcAddr := flag.String("trafficcontroller", "", "The address for trafficcontroller to bind to. (REQUIRED)")
	dopplerAddr := flag.String("doppler", "", "The address for fake doppler to bind to. (REQUIRED)")

	ca := flag.String("ca", "", "The path to the CA. (REQUIRED)")
	cert := flag.String("cert", "", "The path to the server cert. (REQUIRED)")
	key := flag.String("key", "", "The path to the server key. (REQUIRED)")

	iterations := flag.Int("iter", 10000, "The number of envelopes to emit to trafficcontroller.")
	delay := flag.Duration("delay", 2*time.Microsecond, "The delay between envelope emission.")
	cycles := flag.Int("cycles", 5, "The number of tests to run")

	etcdURL := flag.String("etcd", "", "The URL for etcd. (REQUIRED)")
	flag.Parse()

	if *trafficcontrollerCmd == "" {
		log.Fatal("Missing required flag 'cmd'")
	}

	if *tcAddr == "" {
		log.Fatal("Missing required flag 'trafficontroller'")
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

	if *etcdURL == "" {
		log.Fatal("Missing required flag 'etcd'")
	}

	dopplerConfig := &config.Config{
		Zone:            "a-zone",
		JobName:         "benchmarker",
		Index:           "1",
		IncomingUDPPort: 10000,
		OutgoingPort:    8282,
	}

	storeAdapter := connectToEtcd([]string{*etcdURL})
	dopplerservice.Announce(*dopplerAddr, time.Hour*24, dopplerConfig, storeAdapter)

	chunks := strings.SplitN(*trafficcontrollerCmd, " ", -1)

	cmd := exec.Command(chunks[0], chunks[1:]...)
	err := cmd.Start()
	if err != nil {
		log.Panicf("Failed to start trafficcontroller: %s", err)
	}
	defer cmd.Process.Kill()

	creds, err := plumbing.NewServerCredentials(*cert, *key, *ca)
	if err != nil {
		log.Panicf("Unable to setup TLS: %s", err)
	}

	producer := newProducer(int64(*iterations), *delay)

	lis, err := net.Listen("tcp", *dopplerAddr)
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer(grpc.Creds(creds))
	plumbing.RegisterDopplerServer(grpcServer, producer)
	go grpcServer.Serve(lis)

	c := newConsumer()
	go c.Start(*tcAddr)

	var results []int64
	for i := 0; i < *cycles; i++ {
		time.Sleep(10 * time.Second)

		producer.reset <- true
		log.Printf("Total Received: %d", atomic.LoadInt64(&c.count))
		results = append(results, atomic.LoadInt64(&c.count))
		atomic.StoreInt64(&c.count, 0)
		atomic.StoreInt64(&producer.iterations, int64(*iterations))
	}

	fmt.Printf("%s %v\n", *resultTag, results)
}

type producer struct {
	iterations int64
	delay      time.Duration
	reset      chan bool
}

func newProducer(iter int64, delay time.Duration) *producer {
	return &producer{
		iterations: iter,
		delay:      delay,
		reset:      make(chan bool),
	}
}

func (p *producer) Subscribe(r *plumbing.SubscriptionRequest, s plumbing.Doppler_SubscribeServer) error {
	println("Subysubscriber")
	if atomic.LoadInt64(&p.iterations) == 0 {
		<-p.reset
	}

	for atomic.AddInt64(&p.iterations, -1) > 0 {
		// for i := 0; i < p.iterations; i++ {
		s.Send(&plumbing.Response{
			Payload: createEnvelope(),
		})
	}

	return nil
}

func (p *producer) ContainerMetrics(context.Context, *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	return nil, nil
}

func (p *producer) RecentLogs(context.Context, *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	return nil, nil
}

func createEnvelope() []byte {
	e := &events.Envelope{
		Origin:    proto.String("trafficcontroller-benchmark"),
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

func (c *consumer) Start(addr string) {
	consumer := noaa.New(addr, &tls.Config{
		InsecureSkipVerify: true,
	}, nil)
	messages, errs := consumer.Firehose("trafficcontroller-benchmark", "")
	go func() {
		for err := range errs {
			log.Printf("Noaa Error: %s", err)
		}
	}()

	for e := range messages {
		if e.GetOrigin() != "trafficcontroller-benchmark" {
			continue
		}

		atomic.AddInt64(&c.count, 1)
	}
}

func connectToEtcd(etcdURLs []string) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(10)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: etcdURLs,
	}
	options.IsSSL = false
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}
	if err = etcdStoreAdapter.Connect(); err != nil {
		panic(err)
	}
	return etcdStoreAdapter
}
