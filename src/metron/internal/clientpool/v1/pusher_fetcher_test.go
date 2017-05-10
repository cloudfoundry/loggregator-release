package v1_test

import (
	"net"

	"metron/internal/clientpool/v1"
	"plumbing"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PusherFetcher", func() {
	It("opens a stream to the server", func() {
		server := newSpyIngestorServer()
		Expect(server.Start()).To(Succeed())
		defer server.Stop()

		fetcher := v1.NewPusherFetcher(grpc.WithInsecure())
		closer, pusher, err := fetcher.Fetch(server.addr)
		Expect(err).ToNot(HaveOccurred())

		pusher.Send(&plumbing.EnvelopeData{})

		Eventually(server.envelopeData).Should(Receive())
		Expect(closer.Close()).To(Succeed())
	})

	It("returns an error when the server is unavailable", func() {
		fetcher := v1.NewPusherFetcher(grpc.WithInsecure())
		_, _, err := fetcher.Fetch("localhost:1122")
		Expect(err).To(HaveOccurred())
	})
})

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
	lis, err := net.Listen("tcp", ":0")
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
