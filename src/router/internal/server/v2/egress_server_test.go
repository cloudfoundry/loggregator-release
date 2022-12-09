package v2_test

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/metricemitter"
	"code.cloudfoundry.org/loggregator-release/metricemitter/testhelper"
	v2 "code.cloudfoundry.org/loggregator-release/router/internal/server/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ = Describe("EgressServer", func() {
	Describe("Receiver", func() {
		It("returns an unimplemented error code", func() {
			server := v2.NewEgressServer(
				nil,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{}, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("BatchedReceiver", func() {
		It("forwards messages to a connected client", func() {
			ctx, cancel := context.WithCancel(context.Background())
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("context canceled")))
			}()

			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			go func() {
				errors <- server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			}()

			Eventually(spyReceiver.batches).ShouldNot(BeEmpty())
		})

		It("flushes if there are no more envelopes in the diode", func() {
			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("context canceled")))
			}()
			subscriber := &spySubscriber{
				wait: time.Hour,
			}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Hour,
				2000,
			)

			go func() {
				errors <- server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			}()

			Eventually(spyReceiver.batches).Should(HaveLen(1))

			batch := spyReceiver.batches()[0]
			Expect(batch.GetBatch()).To(HaveLen(1))
		})

		It("returns if the context is cancelled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if one occurrs", func() {
			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
				err:      errors.New("some error"),
			}
			defer cancel()
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			Expect(err).To(HaveOccurred())
		})

		It("calls cleanup when exiting", func() {
			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
				err:      errors.New("some error"),
			}
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("some error")))
			}()
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			go func() {
				errors <- server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			}()

			Eventually(subscriber.cleanupCalled).Should(BeTrue())
		})

		It("passes the request to the subscriber", func() {
			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("context canceled")))
			}()
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)

			req := &loggregator_v2.EgressBatchRequest{
				ShardId: "some-shard-id",
			}
			go func() {
				errors <- server.BatchedReceiver(req, spyReceiver)
			}()

			Eventually(subscriber.request).Should(Equal(req))
		})

		It("increments and decrements the subscription count", func() {
			subscriptionsMetric := &metricemitter.Gauge{}
			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			errors := make(chan error)
			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				testhelper.NewMetricClient(),
				&metricemitter.Counter{},
				subscriptionsMetric,
				time.Millisecond,
				10,
			)

			req := &loggregator_v2.EgressBatchRequest{
				ShardId: "some-shard-id",
			}
			go func() {
				errors <- server.BatchedReceiver(req, spyReceiver)
			}()

			Eventually(func() float64 {
				return subscriptionsMetric.GetValue()
			}).Should(Equal(1.0))

			cancel()
			Eventually(errors).Should(Receive(MatchError("context canceled")))

			Eventually(func() float64 {
				return subscriptionsMetric.GetValue()
			}).Should(Equal(0.0))

		})

		It("emits a metric for the number of envelopes sent", func() {
			metricClient := testhelper.NewMetricClient()

			ctx, cancel := context.WithCancel(context.Background())
			spyReceiver := &spyBatchReceiver{
				_context: ctx,
			}
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("context canceled")))
			}()

			subscriber := &spySubscriber{}
			server := v2.NewEgressServer(
				subscriber,
				metricClient,
				&metricemitter.Counter{},
				&metricemitter.Gauge{},
				time.Millisecond,
				10,
			)
			go func() {
				errors <- server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, spyReceiver)
			}()

			Eventually(func() uint64 {
				return metricClient.GetDelta("egress")
			}).Should(BeNumerically(">", 1))
		})

		It("emits a metric for the number of envelopes dropped", func() {
			metricClient := testhelper.NewMetricClient()
			subscriber := &spySubscriber{
				wait: time.Millisecond,
			}

			egressDropped := &metricemitter.Counter{}

			server := v2.NewEgressServer(
				subscriber,
				metricClient,
				egressDropped,
				&metricemitter.Gauge{},
				10*time.Second,
				10,
			)

			ctx, cancel := context.WithCancel(context.Background())
			br := newSlowBatchReciever(100*time.Millisecond, ctx)
			errors := make(chan error)
			defer func() {
				cancel()
				Eventually(errors).Should(Receive(MatchError("context canceled")))
			}()

			go func() {
				errors <- server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, br)
			}()

			Eventually(egressDropped.GetDelta, 10).Should(BeNumerically(">", 1))
		})
	})
})

type slowBatchReceiver struct {
	grpc.ServerStream
	sendDelay time.Duration

	_context context.Context
}

func newSlowBatchReciever(sendDelay time.Duration, ctx context.Context) *slowBatchReceiver {
	return &slowBatchReceiver{
		sendDelay: sendDelay,
		_context:  ctx,
	}
}

func (s *slowBatchReceiver) Send(batch *loggregator_v2.EnvelopeBatch) error {
	time.Sleep(s.sendDelay)
	return nil
}

func (s *slowBatchReceiver) Context() context.Context {
	return s._context
}

type spyBatchReceiver struct {
	grpc.ServerStream

	mu       sync.Mutex
	_batches []*loggregator_v2.EnvelopeBatch
	_context context.Context
	err      error
}

func (s *spyBatchReceiver) Send(batch *loggregator_v2.EnvelopeBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._batches = append(s._batches, batch)

	return s.err
}

func (s *spyBatchReceiver) Context() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s._context
}

func (s *spyBatchReceiver) batches() []*loggregator_v2.EnvelopeBatch {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s._batches
}

type spySubscriber struct {
	mu             sync.Mutex
	_cleanupCalled bool
	_request       *loggregator_v2.EgressBatchRequest

	wait time.Duration
}

func (s *spySubscriber) Subscribe(req *loggregator_v2.EgressBatchRequest, d v2.DataSetter) func() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._request = req
	ctx, cancel := context.WithCancel(context.Background())
	go writeEnvelopes(ctx, d, s.wait)

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		cancel()
		s._cleanupCalled = true
	}
}

func (s *spySubscriber) cleanupCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._cleanupCalled
}

func (s *spySubscriber) request() *loggregator_v2.EgressBatchRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._request
}

func writeEnvelopes(ctx context.Context, d v2.DataSetter, wait time.Duration) {
	var i int
	for {
		i++

		select {
		case <-ctx.Done():
			return
		default:
			d.Set(&loggregator_v2.Envelope{
				SourceId: fmt.Sprintf("%d", i),
			})
			time.Sleep(wait)
		}
	}
}
