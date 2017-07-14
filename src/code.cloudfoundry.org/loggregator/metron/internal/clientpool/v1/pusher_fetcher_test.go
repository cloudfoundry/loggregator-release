package v1_test

import (
	"net"

	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PusherFetcher", func() {
	It("opens a stream to the server", func() {
		server := newSpyIngestorServer()
		Expect(server.Start()).To(Succeed())
		defer server.Stop()

		fetcher := v1.NewPusherFetcher(newSpyRegistry(), grpc.WithInsecure())
		closer, pusher, err := fetcher.Fetch(server.addr)
		Expect(err).ToNot(HaveOccurred())

		pusher.Send(&plumbing.EnvelopeData{})

		Eventually(server.envelopeData).Should(Receive())
		Expect(closer.Close()).To(Succeed())
	})

	It("increments a counter when a connection is established", func() {
		server := newSpyIngestorServer()
		Expect(server.Start()).To(Succeed())
		defer server.Stop()

		registry := newSpyRegistry()

		fetcher := v1.NewPusherFetcher(registry, grpc.WithInsecure())
		_, _, err := fetcher.Fetch(server.addr)
		Expect(err).ToNot(HaveOccurred())

		Expect(registry.GetValue("dopplerConnections")).To(Equal(int64(1)))
		Expect(registry.GetValue("dopplerV1Streams")).To(Equal(int64(1)))
	})

	It("decrements a counter when a connection is closed", func() {
		server := newSpyIngestorServer()
		Expect(server.Start()).To(Succeed())
		defer server.Stop()

		registry := newSpyRegistry()

		fetcher := v1.NewPusherFetcher(registry, grpc.WithInsecure())
		closer, _, err := fetcher.Fetch(server.addr)
		Expect(err).ToNot(HaveOccurred())

		closer.Close()
		Expect(registry.GetValue("dopplerConnections")).To(Equal(int64(0)))
		Expect(registry.GetValue("dopplerV1Streams")).To(Equal(int64(0)))
	})

	It("returns an error when the server is unavailable", func() {
		fetcher := v1.NewPusherFetcher(newSpyRegistry(), grpc.WithInsecure())
		_, _, err := fetcher.Fetch("localhost:1122")
		Expect(err).To(HaveOccurred())
	})
})

type SpyRegistry struct {
	counters map[string]int64
}

func newSpyRegistry() *SpyRegistry {
	return &SpyRegistry{
		counters: make(map[string]int64),
	}
}

func (s *SpyRegistry) Inc(name string) {
	s.counters[name] += 1
}

func (s *SpyRegistry) Dec(name string) {
	s.counters[name] -= 1
}

func (s *SpyRegistry) GetValue(name string) int64 {
	v, ok := s.counters[name]
	if !ok {
		return -89282828
	}

	return v
}

type SpyIngestorServer struct {
	addr         string
	server       *grpc.Server
	stop         chan struct{}
	envelopeData chan *plumbing.EnvelopeData
}

func newSpyIngestorServer() *SpyIngestorServer {
	return &SpyIngestorServer{
		stop:         make(chan struct{}),
		envelopeData: make(chan *plumbing.EnvelopeData),
	}
}

func (s *SpyIngestorServer) Start() error {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}

	s.server = grpc.NewServer()
	s.addr = lis.Addr().String()
	plumbing.RegisterDopplerIngestorServer(s.server, s)

	go s.server.Serve(lis)

	return nil
}

func (s *SpyIngestorServer) Stop() {
	close(s.stop)
	s.server.Stop()
}

func (s *SpyIngestorServer) Pusher(p plumbing.DopplerIngestor_PusherServer) error {
	for {
		select {
		case <-s.stop:
			break
		default:
			env, err := p.Recv()
			if err != nil {
				break
			}

			s.envelopeData <- env
		}
	}

	return nil
}
