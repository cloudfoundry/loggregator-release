package v2_test

import (
	"io"
	"sync"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/router/internal/server/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngressServer", func() {
	var (
		v1Buf                *diodes.ManyToOneEnvelope
		v2Buf                *diodes.ManyToOneEnvelopeV2
		spySenderServer      *spyIngressSender
		spyBatchSenderServer *spyIngressBatchSender
		healthRegistrar      *SpyHealthRegistrar

		ingressMetric *metricemitter.Counter
		ingestor      *v2.IngressServer
	)

	BeforeEach(func() {
		v1Buf = diodes.NewManyToOneEnvelope(5, nil)
		v2Buf = diodes.NewManyToOneEnvelopeV2(5, nil)
		spyBatchSenderServer = newSpyIngressBatchSender(false)
		spySenderServer = newSpyIngressSender(false)
		healthRegistrar = newSpyHealthRegistrar()

		ingressMetric = metricemitter.NewCounter("ingress", "doppler")

		ingestor = v2.NewIngressServer(
			v1Buf,
			v2Buf,
			ingressMetric,
			healthRegistrar,
		)
	})

	It("writes batches to the data setter", func() {
		spyBatchSenderServer.recvCount = 1
		spyBatchSenderServer.envelopes = []*loggregator_v2.Envelope{
			{
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("hello-1"),
					},
				},
			},
			{
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("hello-2"),
					},
				},
			},
		}

		ingestor.BatchSender(spyBatchSenderServer)

		_, ok := v1Buf.TryNext()
		Expect(ok).To(BeTrue())
		_, ok = v2Buf.TryNext()
		Expect(ok).To(BeTrue())

		_, ok = v1Buf.TryNext()
		Expect(ok).To(BeTrue())
		_, ok = v2Buf.TryNext()
		Expect(ok).To(BeTrue())

		Expect(ingressMetric.GetDelta()).To(Equal(uint64(2)))
	})

	It("writes a single envelope to the data setter via stream", func() {
		spySenderServer.recvCount = 1
		spySenderServer.envelope = &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{
					Payload: []byte("hello"),
				},
			},
		}

		ingestor.Sender(spySenderServer)

		_, ok := v1Buf.TryNext()
		Expect(ok).To(BeTrue())
		_, ok = v2Buf.TryNext()
		Expect(ok).To(BeTrue())

		Expect(ingressMetric.GetDelta()).To(Equal(uint64(1)))
	})

	It("finishes modifying the map before it goes on the diode", func() {
		tags := make(map[string]string)

		spySenderServer.recvCount = 1
		spySenderServer.envelope = &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{
					Payload: []byte("hello"),
				},
			},
			Tags: tags,
		}

		go ingestor.Sender(spySenderServer)

		for {
			v2e, ok := v2Buf.TryNext()
			if ok {
				for range v2e.Tags {
					// iterate the map to expose a race
					// this can only fail when run with -race set
				}
				break
			}
		}
	})

	It("throws invalid envelopes on the ground", func() {
		spySenderServer.recvCount = 1
		spySenderServer.envelope = &loggregator_v2.Envelope{}

		ingestor.Sender(spySenderServer)
		_, ok := v1Buf.TryNext()
		Expect(ok).ToNot(BeTrue())
	})

	Describe("health monitoring", func() {
		Describe("Sender()", func() {
			It("increments and decrements the number of ingress streams", func() {
				spySenderServer = newSpyIngressSender(true)
				go ingestor.Sender(spySenderServer)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(1.0))

				close(spySenderServer.done)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(0.0))
			})
		})

		Describe("BatchSender()", func() {
			It("increments and decrements the number of ingress streams", func() {
				spyBatchSenderServer = newSpyIngressBatchSender(true)
				go ingestor.BatchSender(spyBatchSenderServer)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(1.0))

				close(spyBatchSenderServer.done)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(0.0))
			})
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

type spyIngressBatchSender struct {
	loggregator_v2.Ingress_BatchSenderServer

	envelopes []*loggregator_v2.Envelope
	recvCount int
	done      chan struct{}
}

func newSpyIngressBatchSender(blockingRecv bool) *spyIngressBatchSender {
	done := make(chan struct{})

	if !blockingRecv {
		close(done)
	}

	return &spyIngressBatchSender{
		done: done,
	}
}

func (s *spyIngressBatchSender) Recv() (*loggregator_v2.EnvelopeBatch, error) {
	<-s.done

	if s.recvCount == 0 {
		return nil, io.EOF
	}

	s.recvCount--

	return &loggregator_v2.EnvelopeBatch{Batch: s.envelopes}, nil
}

type spyIngressSender struct {
	loggregator_v2.Ingress_SenderServer

	envelope  *loggregator_v2.Envelope
	recvCount int
	done      chan struct{}
}

func newSpyIngressSender(blockingRecv bool) *spyIngressSender {
	done := make(chan struct{})

	if !blockingRecv {
		close(done)
	}

	return &spyIngressSender{
		done: done,
	}
}

func (s *spyIngressSender) Recv() (*loggregator_v2.Envelope, error) {
	<-s.done

	if s.recvCount == 0 {
		return nil, io.EOF
	}

	s.recvCount--

	return s.envelope, nil
}
