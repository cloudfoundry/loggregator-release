package v2_test

import (
	"io"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/diodes"
	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	v2 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngressServer", func() {
	var (
		v1Buf                *diodes.ManyToOneEnvelope
		v2Buf                *diodes.ManyToOneEnvelopeV2
		spySenderServer      *spyIngressSender
		spyBatchSenderServer *spyIngressBatchSender

		ingressMetric *metricemitter.Counter
		ingestor      *v2.IngressServer
	)

	BeforeEach(func() {
		v1Buf = diodes.NewManyToOneEnvelope(5, nil)
		v2Buf = diodes.NewManyToOneEnvelopeV2(5, nil)
		spyBatchSenderServer = newSpyIngressBatchSender(false)
		spySenderServer = newSpyIngressSender(false)

		ingressMetric = metricemitter.NewCounter("ingress", "doppler")

		ingestor = v2.NewIngressServer(
			v1Buf,
			v2Buf,
			ingressMetric,
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

		err := ingestor.BatchSender(spyBatchSenderServer)
		Expect(err).To(Equal(io.EOF))

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

		err := ingestor.Sender(spySenderServer)
		Expect(err).To(Equal(io.EOF))

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

		go func() {
			err := ingestor.Sender(spySenderServer)
			Expect(err).To(Equal(io.EOF))
		}()

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

		err := ingestor.Sender(spySenderServer)
		Expect(err).To(Equal(io.EOF))
		_, ok := v1Buf.TryNext()
		Expect(ok).ToNot(BeTrue())
	})
})

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
