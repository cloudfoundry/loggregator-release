package v1_test

import (
	"errors"
	"io"
	"net"
	"sync"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v1"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngestorServer", func() {
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
		v1Buf           *diodes.ManyToOneEnvelope
		v2Buf           *diodes.ManyToOneEnvelopeV2
		manager         *v1.IngestorServer
		server          *grpc.Server
		connCloser      io.Closer
		dopplerClient   plumbing.DopplerIngestorClient
		healthRegistrar *SpyHealthRegistrar
	)

	BeforeEach(func() {
		var grpcAddr string
		v1Buf = diodes.NewManyToOneEnvelope(5, nil)
		v2Buf = diodes.NewManyToOneEnvelopeV2(5, nil)
		mockBatcher := newSpyBatcher()
		mockChainer := newSpyBatchCounterChainer()
		mockBatcher.batchCounter = mockChainer
		mockChainer.setTag = mockChainer
		healthRegistrar = newSpyHealthRegistrar()

		manager = v1.NewIngestorServer(v1Buf, v2Buf, mockBatcher, healthRegistrar)
		server, grpcAddr = startGRPCServer(manager)
		dopplerClient, connCloser = establishClient(grpcAddr)
	})

	AfterEach(func() {
		server.Stop()
		connCloser.Close()
	})

	It("reads envelopes from ingestor client into the v1 Buffer", func() {
		pusherClient, err := dopplerClient.Pusher(context.TODO())
		Expect(err).ToNot(HaveOccurred())

		someEnvelope, data := buildContainerMetric()
		pusherClient.Send(&plumbing.EnvelopeData{data})

		f := func() *events.Envelope {
			env, ok := v1Buf.TryNext()
			if !ok {
				return nil
			}

			return env
		}
		Eventually(f).Should(Equal(someEnvelope))
	})

	It("reads envelopes from ingestor client into the v2 Buffer", func() {
		pusherClient, err := dopplerClient.Pusher(context.TODO())
		Expect(err).ToNot(HaveOccurred())

		someEnvelope, data := buildContainerMetric()
		pusherClient.Send(&plumbing.EnvelopeData{data})

		v2e := conversion.ToV2(someEnvelope, true)
		Eventually(v2Buf.Next).Should(Equal(v2e))
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
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())

			err = pusherClient.Send(&plumbing.EnvelopeData{nil})
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an EOF error", func() {
		It("exits the function gracefully", func() {
			fakeStream := newSpyIngestorGRPCServer()
			fakeStream.recvError = io.EOF

			Eventually(func() error {
				return manager.Pusher(fakeStream)
			}).Should(Succeed())
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an error", func() {
		It("does not forward the message to the sender", func() {
			fakeStream := newSpyIngestorGRPCServer()
			fakeStream.recvError = errors.New("fake error")

			go manager.Pusher(fakeStream)
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the pusher context finishes", func() {
		It("returns the error from the context", func() {
			fakeStream := newSpyIngestorGRPCServer()

			for i := 0; i < 100; i++ {
				fakeStream.recvError = errors.New("fake error")
			}

			context, cancelCtx := context.WithCancel(context.Background())
			fakeStream.context = context
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

func newSpyBatcher() *spyBatcher {
	return &spyBatcher{}
}

type spyBatcher struct {
	batchCounter interface{} // FIXME
}

func (s *spyBatcher) BatchCounter(name string) metricbatcher.BatchCounterChainer {
	return &spyBatchCounterChainer{}
}

func newSpyBatchCounterChainer() *spyBatchCounterChainer {
	return &spyBatchCounterChainer{}
}

type spyBatchCounterChainer struct {
	setTag interface{} // FIXME
}

func (s *spyBatchCounterChainer) Add(value uint64) {
}

func (s *spyBatchCounterChainer) Increment() {
}

func (s *spyBatchCounterChainer) SetTag(key, value string) metricbatcher.BatchCounterChainer {
	return s
}

func newSpyIngestorGRPCServer() *spyIngestorGRPCServer {
	return &spyIngestorGRPCServer{}
}

type spyIngestorGRPCServer struct {
	context          context.Context
	recvEnvelopeData *plumbing.EnvelopeData
	recvError        error
	grpc.ServerStream
}

func (s *spyIngestorGRPCServer) Context() context.Context {
	if s.context == nil {
		return context.TODO()
	}
	return s.context
}

func (s *spyIngestorGRPCServer) SendAndClose(r *plumbing.PushResponse) error {
	return nil
}

func (s *spyIngestorGRPCServer) Recv() (*plumbing.EnvelopeData, error) {
	return s.recvEnvelopeData, s.recvError
}
