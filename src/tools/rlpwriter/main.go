// rlpwriter: a tool that writes messages into RLP.
//
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/plumbing"
)

var (
	grpcAddr = flag.String("grpc-addr", "localhost:3457", "the host:port for gRPC to listen")
	httpAddr = flag.String("http-addr", "localhost:8080", "the host:port for HTTP to listen")
	certFile = flag.String("cert", "", "server cert")
	keyFile  = flag.String("key", "", "server key")
	caFile   = flag.String("ca", "", "ca cert used to validate client certs")
	delay    = flag.Duration("delay", time.Second, "delay inbetween sending messages")
)

var msg1K []byte = []byte(strings.Repeat("J", 1024))

func main() {
	flag.Parse()

	creds, err := plumbing.NewServerCredentials(
		*certFile,
		*keyFile,
		*caFile,
	)
	if err != nil {
		log.Fatal(err)
	}

	lis, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.Creds(creds))
	srv := newServer()
	plumbing.RegisterDopplerServer(s, srv)

	go func() {
		log.Fatal(s.Serve(lis))
	}()
	log.Fatal(http.ListenAndServe(*httpAddr, srv))
}

type server struct {
	plumbing.DopplerServer
	mu     sync.Mutex
	report map[string]int64
}

func newServer() *server {
	return &server{
		report: make(map[string]int64),
	}
}

func (s *server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	json.NewEncoder(rw).Encode(s.report)
}

func (s *server) set(appID string, v int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.report[appID] = v
}

func (s *server) Subscribe(req *plumbing.SubscriptionRequest, sub plumbing.Doppler_SubscribeServer) error {
	var lastIdx int64
	for {
		e := &events.Envelope{
			Origin:    proto.String("rlpwriter"),
			EventType: events.Envelope_LogMessage.Enum(),
			Tags: map[string]string{
				"idx": strconv.FormatInt(lastIdx, 10),
			},
			LogMessage: &events.LogMessage{
				Message:     msg1K,
				MessageType: events.LogMessage_OUT.Enum(),
				Timestamp:   proto.Int64(1234),
				AppId:       &req.GetFilter().AppID,
			},
		}
		data, err := proto.Marshal(e)
		if err != nil {
			log.Fatalf("error while marshalling: %s", err)
		}
		err = sub.Send(&plumbing.Response{Payload: data})
		if err != nil {
			return nil
		}
		s.set(req.GetFilter().AppID, lastIdx)
		lastIdx++
		time.Sleep(*delay)
	}
	return nil
}
