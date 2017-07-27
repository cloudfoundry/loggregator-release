package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
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
	metronCmd := flag.String("cmd", "", "The command to start metron. (REQUIRED)")
	dopplerAddr := flag.String("doppler", "", "The address for fake doppler to bind to. (REQUIRED)")
	metronAddr := flag.String("metron", "", "The address metron is listening to. (REQUIRED)")

	ca := flag.String("ca", "", "The path to the CA. (REQUIRED)")
	cert := flag.String("cert", "", "The path to the server cert. (REQUIRED)")
	key := flag.String("key", "", "The path to the server key. (REQUIRED)")

	iterations := flag.Int("iter", 10000, "The number of envelopes to emit to metron.")
	delay := flag.Duration("delay", 2*time.Microsecond, "The delay between envelope emission.")
	cycles := flag.Int("cycles", 5, "The number of tests to run")
	flag.Parse()

	if *metronCmd == "" {
		log.Fatal("Missing required flag 'cmd'")
	}

	if *dopplerAddr == "" {
		log.Fatal("Missing required flag 'doppler'")
	}

	if *metronAddr == "" {
		log.Fatal("Missing required flag 'metron'")
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

	certs, err := plumbing.NewServerCredentials(*cert, *key, *ca)
	if err != nil {
		log.Fatalf("Unable to setup TLS: %s", err)
	}

	c := newConsumer()
	consumeEnvelopes(c, *dopplerAddr, certs)

	chunks := strings.SplitN(*metronCmd, " ", -1)

	cmd := exec.Command(chunks[0], chunks[1:]...)
	err = cmd.Start()
	if err != nil {
		log.Panicf("Failed to start metron: %s", err)
	}
	defer cmd.Process.Kill()

	var results []int64
	for i := 0; i < *cycles; i++ {
		time.Sleep(5 * time.Second)
		atomic.StoreInt64(&c.count, 0)

		produceEnvelopes(*iterations, *delay, *metronAddr)

		time.Sleep(5 * time.Second)

		log.Printf("Total Received: %d", atomic.LoadInt64(&c.count))
		results = append(results, atomic.LoadInt64(&c.count))
	}

	fmt.Printf("%s %v\n", *resultTag, results)
}

func consumeEnvelopes(c *consumer, addr string, creds credentials.TransportCredentials) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panicf("Failed to listen: %s", err)
	}

	server := grpc.NewServer(grpc.Creds(creds))
	plumbing.RegisterDopplerIngestorServer(server, c)
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Panicf("Failed to serve: %s", err)
		}
	}()
}

func produceEnvelopes(iterations int, delay time.Duration, metronAddr string) {
	conn := dial(metronAddr)

	for i := 0; i < iterations; i++ {
		_, err := conn.Write(createEnvelope())
		if err != nil {
			log.Panicf("Failed to write data to metron: %s", err)
		}

		time.Sleep(delay)
	}
}

func createEnvelope() []byte {
	e := &events.Envelope{
		Origin:    proto.String("metron-benchmark"),
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

func dial(addr string) *net.UDPConn {
	raddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Panicf("Unable to resolve addr (%s): %s", addr, err)
	}

	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		log.Panicf("Unable to dial: %s", err)
	}

	return conn
}

type consumer struct {
	count int64
}

func newConsumer() *consumer {
	return &consumer{}
}

func (c *consumer) Pusher(s plumbing.DopplerIngestor_PusherServer) error {
	for {
		data, err := s.Recv()
		if err != nil {
			log.Printf("Failed while receiving: %s", err)
			return err
		}

		var e events.Envelope
		err = proto.Unmarshal(data.Payload, &e)
		if err != nil {
			log.Panicf("Failed to unmarshal received envelope: %s", err)
		}

		if e.GetOrigin() != "metron-benchmark" {
			continue
		}

		atomic.AddInt64(&c.count, 1)

		// fmt.Printf("%+v\n", e)
	}
	return nil
}
