package egress_test

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/rlp/internal/egress"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	Describe("Receiver()", func() {
		It("errors when the sender cannot send the envelope", func() {
			receiverServer := &spyReceiverServer{err: errors.New("Oh No!")}
			receiver := newSpyReceiver(1)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&v2.EgressRequest{}, receiverServer)

			Expect(err).To(Equal(io.ErrUnexpectedEOF))
		})

		It("streams data when there are envelopes", func() {
			receiverServer := &spyReceiverServer{}
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&v2.EgressRequest{}, receiverServer)
			Expect(err).ToNot(HaveOccurred())
			Eventually(receiverServer.EnvelopeCount).Should(Equal(int64(10)))
		})

		It("forwards the request as an egress batch request", func() {
			receiverServer := &spyReceiverServer{}
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			egressReq := &v2.EgressRequest{
				ShardId: "a-shard-id",
				Selectors: []*v2.Selector{
					{SourceId: "a-source-id"},
				},
				UsePreferredTags: true,
			}
			err := server.Receiver(egressReq, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var egressBatchReq *v2.EgressBatchRequest
			Eventually(receiver.requests).Should(Receive(&egressBatchReq))
			Expect(egressBatchReq.GetShardId()).To(Equal(egressReq.GetShardId()))
			Expect(egressBatchReq.GetSelectors()).To(Equal(egressReq.GetSelectors()))
			Expect(egressBatchReq.GetUsePreferredTags()).To(Equal(egressReq.GetUsePreferredTags()))
		})

		It("errors when it receives a legacy selector and a selector", func() {
			receiverServer := &spyReceiverServer{}
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&v2.EgressRequest{
				LegacySelector: &v2.Selector{
					SourceId: "source-id",
				},
				Selectors: []*v2.Selector{
					{SourceId: "source-id"},
				},
			}, receiverServer)

			Expect(err).To(MatchError("Using LegacySelector with Selectors is not supported."))
		})

		It("closes the receiver when the context is canceled", func() {
			receiverServer := &spyReceiverServer{}
			receiver := newSpyReceiver(1000000000)
			ctx, cancel := context.WithCancel(context.TODO())
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				ctx,
				1,
				time.Nanosecond,
			)

			go func() {
				err := server.Receiver(&v2.EgressRequest{}, receiverServer)
				Expect(err).ToNot(HaveOccurred())
			}()

			cancel()

			var rxCtx context.Context
			Eventually(receiver.ctx).Should(Receive(&rxCtx))
			Eventually(rxCtx.Done).Should(BeClosed())
		})

		It("cancels the context when Receiver exits", func() {
			receiverServer := &spyReceiverServer{
				err: errors.New("Oh no!"),
			}
			receiver := newSpyReceiver(100000000)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			go server.Receiver(&v2.EgressRequest{}, receiverServer)

			var ctx context.Context
			Eventually(receiver.ctx).Should(Receive(&ctx))
			Eventually(ctx.Done()).Should(BeClosed())
		})

		Describe("Metrics", func() {
			It("emits 'egress' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := &spyReceiverServer{}
				receiver := newSpyReceiver(10)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)

				err := server.Receiver(&v2.EgressRequest{}, receiverServer)

				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(BeNumerically("==", 10))
			})

			It("emits 'dropped' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := &spyReceiverServer{
					wait: make(chan struct{}),
				}
				defer receiverServer.stopWait()

				receiver := newSpyReceiver(1000000)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.Receiver(&v2.EgressRequest{}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("dropped")
				}, 3).Should(BeNumerically(">", 100))
			})
		})

		Describe("health monitoring", func() {
			It("increments and decrements subscription count", func() {
				receiverServer := &spyReceiverServer{}
				receiver := newSpyReceiver(1000000000)

				health := newSpyHealthRegistrar()
				server := egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					health,
					context.TODO(),
					1,
					time.Nanosecond,
				)
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

	Describe("BatchedReceiver()", func() {
		It("errors when the sender cannot send the envelope", func() {
			receiverServer := &spyBatchedReceiverServer{err: errors.New("Oh No!")}
			server := egress.NewServer(
				&stubReceiver{},
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&v2.EgressBatchRequest{}, receiverServer)

			Expect(err).To(Equal(io.ErrUnexpectedEOF))
		})

		It("streams data when there are envelopes", func() {
			receiverServer := &spyBatchedReceiverServer{}
			server := egress.NewServer(
				newSpyReceiver(10),
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				10,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&v2.EgressBatchRequest{}, receiverServer)

			Expect(err).ToNot(HaveOccurred())
			Eventually(receiverServer.EnvelopeCount).Should(Equal(int64(10)))
		})

		It("errors when it receives a legacy selector and a selector", func() {
			receiverServer := &spyBatchedReceiverServer{}
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&v2.EgressBatchRequest{
				LegacySelector: &v2.Selector{
					SourceId: "source-id",
				},
				Selectors: []*v2.Selector{
					{SourceId: "source-id"},
				},
			}, receiverServer)

			Expect(err).To(MatchError("Using LegacySelector with Selectors is not supported."))
		})

		It("passes the egress batch request through", func() {
			receiverServer := &spyBatchedReceiverServer{}
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			expectedReq := &v2.EgressBatchRequest{
				ShardId: "a-shard-id",
				Selectors: []*v2.Selector{
					{SourceId: "a-source-id"},
				},
				UsePreferredTags: true,
			}
			err := server.BatchedReceiver(expectedReq, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			Eventually(receiver.requests).Should(Receive(Equal(expectedReq)))
		})

		It("closes the receiver when the context is canceled", func() {
			receiverServer := &spyBatchedReceiverServer{}
			receiver := newSpyReceiver(1000000000)

			ctx, cancel := context.WithCancel(context.TODO())
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				ctx,
				1,
				time.Nanosecond,
			)

			go func() {
				err := server.BatchedReceiver(&v2.EgressBatchRequest{}, receiverServer)
				Expect(err).ToNot(HaveOccurred())
			}()

			cancel()

			var rxCtx context.Context
			Eventually(receiver.ctx).Should(Receive(&rxCtx))
			Eventually(rxCtx.Done).Should(BeClosed())
		})

		It("cancels the context when Receiver exits", func() {
			receiverServer := &spyBatchedReceiverServer{
				err: errors.New("Oh no!"),
			}
			receiver := newSpyReceiver(100000000)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)
			go server.BatchedReceiver(&v2.EgressBatchRequest{}, receiverServer)

			var ctx context.Context
			Eventually(receiver.ctx).Should(Receive(&ctx))
			Eventually(ctx.Done()).Should(BeClosed())
		})

		Describe("Metrics", func() {
			It("emits 'egress' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiver := newSpyReceiver(10)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					10,
					time.Second,
				)

				err := server.BatchedReceiver(
					&v2.EgressBatchRequest{},
					&spyBatchedReceiverServer{},
				)

				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(BeNumerically("==", 10))
			})

			It("emits 'dropped' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := &spyBatchedReceiverServer{}
				receiver := newSpyReceiver(1000000)

				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)
				go server.BatchedReceiver(&v2.EgressBatchRequest{}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("dropped")
				}, 3).Should(BeNumerically(">", 100))
			})
		})

		Describe("health monitoring", func() {
			It("increments and decrements subscription count", func() {
				receiver := newSpyReceiver(1000000000)
				health := newSpyHealthRegistrar()
				server := egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					health,
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.BatchedReceiver(
					&v2.EgressBatchRequest{},
					&spyBatchedReceiverServer{},
				)

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
	wait          chan struct{}

	grpc.ServerStream
}

func (*spyReceiverServer) Context() context.Context {
	return context.Background()
}

func (s *spyReceiverServer) Send(*v2.Envelope) error {
	if s.wait != nil {
		<-s.wait
		return nil
	}

	atomic.AddInt64(&s.envelopeCount, 1)
	return s.err
}

func (s *spyReceiverServer) EnvelopeCount() int64 {
	return atomic.LoadInt64(&s.envelopeCount)
}

func (s *spyReceiverServer) stopWait() {
	close(s.wait)
}

type spyBatchedReceiverServer struct {
	err           error
	envelopeCount int64

	grpc.ServerStream
}

func (*spyBatchedReceiverServer) Context() context.Context {
	return context.Background()
}

func (s *spyBatchedReceiverServer) Send(b *v2.EnvelopeBatch) error {
	atomic.AddInt64(&s.envelopeCount, int64(len(b.Batch)))
	return s.err
}

func (s *spyBatchedReceiverServer) EnvelopeCount() int64 {
	return atomic.LoadInt64(&s.envelopeCount)
}

type spyReceiver struct {
	envelope       *v2.Envelope
	envelopeRepeat int

	stopCh   chan struct{}
	ctx      chan context.Context
	requests chan *v2.EgressBatchRequest
}

func newSpyReceiver(envelopeCount int) *spyReceiver {
	return &spyReceiver{
		envelope:       &v2.Envelope{},
		envelopeRepeat: envelopeCount,
		stopCh:         make(chan struct{}),
		ctx:            make(chan context.Context, 1),
		requests:       make(chan *v2.EgressBatchRequest, 100),
	}
}

func (s *spyReceiver) Subscribe(ctx context.Context, req *v2.EgressBatchRequest) (func() (*v2.Envelope, error), error) {
	s.ctx <- ctx
	s.requests <- req

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

type stubReceiver struct{}

func (s *stubReceiver) Subscribe(ctx context.Context, req *v2.EgressBatchRequest) (func() (*v2.Envelope, error), error) {
	rx := func() (*v2.Envelope, error) {
		return &v2.Envelope{}, nil
	}
	return rx, nil
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
