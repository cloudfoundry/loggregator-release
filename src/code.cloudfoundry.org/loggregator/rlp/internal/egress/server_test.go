package egress_test

import (
	"errors"
	"io"
	"metricemitter/testhelper"
	"sync"
	"sync/atomic"

	"code.cloudfoundry.org/loggregator/rlp/internal/egress"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	var (
		receiver       *spyReceiver
		receiverServer *spyReceiverServer
		server         *egress.Server
		ctx            context.Context
		metricClient   *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		metricClient = testhelper.NewMetricClient()
	})

	Describe("Receiver()", func() {
		It("returns an error for a request that has type filter but not a source ID", func() {
			req := &v2.EgressRequest{
				Filter: &v2.Filter{
					Message: &v2.Filter_Log{
						Log: &v2.LogFilter{},
					},
				},
			}
			receiverServer = &spyReceiverServer{}
			receiver = &spyReceiver{}
			server = egress.NewServer(receiver, metricClient, newSpyHealthRegistrar())

			err := server.Receiver(req, receiverServer)
			Expect(err).To(MatchError("invalid request: cannot have type filter without source id"))
		})

		It("errors when the sender cannot send the envelope", func() {
			receiverServer = &spyReceiverServer{err: errors.New("Oh No!")}
			receiver = &spyReceiver{
				envelope:       &v2.Envelope{},
				envelopeRepeat: 1,
			}

			server = egress.NewServer(receiver, metricClient, newSpyHealthRegistrar())
			err := server.Receiver(&v2.EgressRequest{}, receiverServer)
			Expect(err).To(Equal(io.ErrUnexpectedEOF))
		})

		It("streams data when there are envelopes", func() {
			receiverServer = &spyReceiverServer{}
			receiver = &spyReceiver{
				envelope:       &v2.Envelope{},
				envelopeRepeat: 10,
			}

			server = egress.NewServer(receiver, metricClient, newSpyHealthRegistrar())
			server.Receiver(&v2.EgressRequest{}, receiverServer)

			Eventually(receiverServer.EnvelopeCount).Should(Equal(int64(10)))
		})

		Describe("Metrics", func() {
			It("emits 'egress' metric for each envelope", func() {
				receiverServer = &spyReceiverServer{}
				receiver = &spyReceiver{
					envelope:       &v2.Envelope{},
					envelopeRepeat: 10,
				}

				server = egress.NewServer(receiver, metricClient, newSpyHealthRegistrar())
				server.Receiver(&v2.EgressRequest{}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(BeNumerically("==", 10))
			})

			It("emits 'dropped' metric for each envelope", func() {
				receiverServer = &spyReceiverServer{}
				receiver = &spyReceiver{
					envelope:       &v2.Envelope{},
					envelopeRepeat: 1000000,
				}

				server = egress.NewServer(receiver, metricClient, newSpyHealthRegistrar())
				server.Receiver(&v2.EgressRequest{}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("dropped")
				}).Should(BeNumerically(">", 100))
			})
		})

		Describe("health monitoring", func() {
			It("increments and decrements subscription count", func() {
				receiverServer = &spyReceiverServer{}
				receiver = &spyReceiver{
					envelope:       &v2.Envelope{},
					envelopeRepeat: 1000000000,
					stopCh:         make(chan struct{}),
				}

				health := newSpyHealthRegistrar()
				server = egress.NewServer(receiver, metricClient, health)
				go server.Receiver(&v2.EgressRequest{}, receiverServer)

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(1.0))

				receiver.stop()

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(0.0))
			})
		})
	})
})

type spyReceiverServer struct {
	err           error
	envelopeCount int64

	grpc.ServerStream
}

func (*spyReceiverServer) Context() context.Context {
	return context.Background()
}

func (s *spyReceiverServer) Send(*v2.Envelope) error {
	atomic.AddInt64(&s.envelopeCount, 1)
	return s.err
}

func (s *spyReceiverServer) EnvelopeCount() int64 {
	return atomic.LoadInt64(&s.envelopeCount)
}

type spyReceiver struct {
	envelope       *v2.Envelope
	envelopeRepeat int

	stopCh chan struct{}
}

func (s *spyReceiver) Receive(ctx context.Context, req *v2.EgressRequest) (func() (*v2.Envelope, error), error) {
	return func() (*v2.Envelope, error) {
		if s.envelopeRepeat > 0 {
			select {
			case <-s.stopCh:
				return nil, io.EOF
			default:
				s.envelopeRepeat--
				return s.envelope, nil
			}
		}

		return nil, errors.New("Oh no!")
	}, nil
}

func (s *spyReceiver) stop() {
	close(s.stopCh)
}

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
