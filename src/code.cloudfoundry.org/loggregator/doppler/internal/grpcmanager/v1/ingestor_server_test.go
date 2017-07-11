package v1_test

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"

	"google.golang.org/grpc"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngestorManager", func() {
	var startGRPCServer = func(ds plumbing.DopplerIngestorServer) (*grpc.Server, string) {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		s := grpc.NewServer()
		plumbing.RegisterDopplerIngestorServer(s, ds)
		go s.Serve(lis)

		return s, lis.Addr().String()
	}

	var establishClient = func(dopplerAddr string) (plumbing.DopplerIngestorClient, io.Closer) {
		conn, err := grpc.Dial(dopplerAddr, grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		c := plumbing.NewDopplerIngestorClient(conn)

		return c, conn
	}

	var (
		outgoingMsgs    *diodes.ManyToOneEnvelope
		manager         *v1.IngestorServer
		server          *grpc.Server
		connCloser      io.Closer
		dopplerClient   plumbing.DopplerIngestorClient
		healthRegistrar *SpyHealthRegistrar
	)

	BeforeEach(func() {
		var grpcAddr string
		outgoingMsgs = diodes.NewManyToOneEnvelope(5, nil)
		mockBatcher := newMockBatcher()
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)
		healthRegistrar = newSpyHealthRegistrar()

		manager = v1.NewIngestorServer(outgoingMsgs, mockBatcher, healthRegistrar)
		server, grpcAddr = startGRPCServer(manager)
		dopplerClient, connCloser = establishClient(grpcAddr)
	})

	AfterEach(func() {
		server.Stop()
		connCloser.Close()
	})

	It("reads envelopes from ingestor client", func() {
		pusherClient, err := dopplerClient.Pusher(context.TODO())
		Expect(err).ToNot(HaveOccurred())

		someEnvelope, data := buildContainerMetric()
		pusherClient.Send(&plumbing.EnvelopeData{data})

		Eventually(outgoingMsgs.Next).Should(Equal(someEnvelope))
	})

	Describe("health monitoring", func() {
		It("increments and decrements the number of ingress streams", func() {
			pusher, err := dopplerClient.Pusher(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			_, data := buildContainerMetric()
			pusher.Send(&plumbing.EnvelopeData{data})

			Eventually(func() float64 {
				return healthRegistrar.Get("ingressStreamCount")
			}).Should(Equal(1.0))

			pusher.CloseAndRecv()

			Eventually(func() float64 {
				return healthRegistrar.Get("ingressStreamCount")
			}).Should(Equal(0.0))
		})
	})

	Context("With an unsupported envelope payload", func() {
		It("does not forward the message to the sender", func() {
			pusherClient, err := dopplerClient.Pusher(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			err = pusherClient.Send(&plumbing.EnvelopeData{[]byte("unsupported envelope")})
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				_, ok := outgoingMsgs.TryNext()
				return ok
			}).Should(BeFalse())

			err = pusherClient.Send(&plumbing.EnvelopeData{nil})
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				_, ok := outgoingMsgs.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an EOF error", func() {
		It("exits the function gracefully", func() {
			fakeStream := newMockIngestorGRPCServer()
			fakeStream.RecvOutput.Ret0 <- nil
			fakeStream.RecvOutput.Ret1 <- io.EOF
			fakeStream.ContextOutput.Ret0 <- context.TODO()

			Eventually(func() error {
				return manager.Pusher(fakeStream)
			}).Should(Succeed())
			Consistently(func() bool {
				_, ok := outgoingMsgs.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an error", func() {
		It("does not forward the message to the sender", func() {
			fakeStream := newMockIngestorGRPCServer()
			fakeStream.RecvOutput.Ret0 <- nil
			fakeStream.RecvOutput.Ret1 <- errors.New("fake error")
			fakeStream.ContextOutput.Ret0 <- context.TODO()

			go manager.Pusher(fakeStream)
			Consistently(func() bool {
				_, ok := outgoingMsgs.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the pusher context finishes", func() {
		It("returns the error from the context", func() {
			fakeStream := newMockIngestorGRPCServer()

			for i := 0; i < 100; i++ {
				fakeStream.RecvOutput.Ret0 <- nil
				fakeStream.RecvOutput.Ret1 <- errors.New("fake error")
			}

			context, cancelCtx := context.WithCancel(context.Background())
			fakeStream.ContextOutput.Ret0 <- context
			cancelCtx()

			err := manager.Pusher(fakeStream)

			Expect(err).To(HaveOccurred())
		})
	})
})

type SpyHealthRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
}

func newSpyHealthRegistrar() *SpyHealthRegistrar {
	return &SpyHealthRegistrar{
		values: make(map[string]float64),
	}
}

func (s *SpyHealthRegistrar) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]++
}

func (s *SpyHealthRegistrar) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]--
}

func (s *SpyHealthRegistrar) Get(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}
