// metrondump: a tool for receiving messages from metron via the v1 gRPC
// protocol. It is meant to run with the udpwriter tool.

// Messages that have the origin "fast" will be aggregated and a rate will
// print every second. Messages that have the origin "slow" will be printed
// out immediately with their latency. All other messages will be thrown away.
//
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	port     = flag.Int("port", 12345, "port to use to listen for gRPC v1 messages")
	certFile = flag.String("cert", "", "cert to use to listen for gRPC v1 messages")
	keyFile  = flag.String("key", "", "key to use to listen for gRPC v1 messages")
	caFile   = flag.String("ca", "", "ca cert to use to listen for gRPC v1 messages")
)

func main() {
	flag.Parse()

	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		*certFile,
		*keyFile,
		*caFile,
	)
	if err != nil {
		log.Fatal(err)
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(transportCreds))
	plumbing.RegisterDopplerIngestorServer(
		grpcServer,
		&Server{},
	)
	log.Printf("Starting gRPC server on %s", listener.Addr().String())
	log.Fatal(grpcServer.Serve(listener))
}

type Server struct{}

func (s *Server) Pusher(server plumbing.DopplerIngestor_PusherServer) error {
	name := randString()

	var fastRate uint64
	stop := make(chan bool)
	defer close(stop)
	go printRate(&fastRate, name, stop)

	for {
		envData, err := server.Recv()
		if err != nil {
			return nil
		}
		var env events.Envelope
		err = proto.Unmarshal(envData.Payload, &env)
		if err != nil {
			log.Print(err)
			continue
		}
		tripTime := time.Since(time.Unix(0, *env.Timestamp))
		if *env.Origin == "slow" {
			fmt.Printf("%s: slow: %s\n", name, tripTime)
		}
		if *env.Origin == "fast" {
			atomic.AddUint64(&fastRate, 1)
		}
	}
}

func printRate(fastRate *uint64, name string, stop chan bool) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		rate := atomic.SwapUint64(fastRate, 0)
		fmt.Printf("%s: fast-rate: %d\\s\n", name, rate)
		time.Sleep(time.Second)
	}
}

func randString() string {
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		log.Panicf("unable to read randomness %s:", err)
	}
	return fmt.Sprintf("%x", b)
}
