package v2_test

import (
	"errors"
	"io"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Receiver", func() {
	var (
		rx *ingress.Receiver

		spySetter *SpySetter
	)

	BeforeEach(func() {
		spySetter = NewSpySetter()

		rx = ingress.NewReceiver(spySetter, testhelper.NewMetricClient())
	})

	Describe("Sender()", func() {
		var (
			spySender *SpySender
		)

		BeforeEach(func() {
			spySender = NewSpySender()
		})

		It("calls set on the data setter with the data", func() {
			e := &v2.Envelope{
				SourceId: "some-id",
			}

			spySender.recvResponses <- SenderRecvResponse{
				envelope: e,
			}
			spySender.recvResponses <- SenderRecvResponse{
				envelope: e,
			}
			spySender.recvResponses <- SenderRecvResponse{
				err: io.EOF,
			}

			rx.Sender(spySender)

			Expect(spySetter.envelopes).To(Receive(Equal(e)))
			Expect(spySetter.envelopes).To(Receive(Equal(e)))
		})

		It("returns an error when receive fails", func() {
			spySender.recvResponses <- SenderRecvResponse{
				err: errors.New("error occurred"),
			}

			err := rx.Sender(spySender)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("BatchSender()", func() {
		var (
			spyBatchSender *SpyBatchSender
		)

		BeforeEach(func() {
			spyBatchSender = NewSpyBatchSender()
		})

		It("calls set on the datasetting with all the envelopes", func() {
			e := &v2.Envelope{
				SourceId: "some-id",
			}

			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				envelopes: []*v2.Envelope{e, e, e, e, e},
			}
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: io.EOF,
			}

			rx.BatchSender(spyBatchSender)

			Expect(spySetter.envelopes).Should(HaveLen(5))
		})

		It("returns an error when receive fails", func() {
			spyBatchSender.recvResponses <- BatchSenderRecvResponse{
				err: errors.New("error occurred"),
			}

			err := rx.BatchSender(spyBatchSender)

			Expect(err).To(HaveOccurred())
		})
	})
})

type SenderRecvResponse struct {
	envelope *v2.Envelope
	err      error
}

type BatchSenderRecvResponse struct {
	envelopes []*v2.Envelope
	err       error
}

type SpySender struct {
	v2.Ingress_SenderServer
	recvResponses chan SenderRecvResponse
}

func NewSpySender() *SpySender {
	return &SpySender{
		recvResponses: make(chan SenderRecvResponse, 100),
	}
}

func (s *SpySender) Recv() (*v2.Envelope, error) {
	resp := <-s.recvResponses

	return resp.envelope, resp.err
}

type SpyBatchSender struct {
	v2.Ingress_BatchSenderServer
	recvResponses chan BatchSenderRecvResponse
}

func NewSpyBatchSender() *SpyBatchSender {
	return &SpyBatchSender{
		recvResponses: make(chan BatchSenderRecvResponse, 100),
	}
}

func (s *SpyBatchSender) Recv() (*v2.EnvelopeBatch, error) {
	resp := <-s.recvResponses

	return &v2.EnvelopeBatch{Batch: resp.envelopes}, resp.err
}

type SpySetter struct {
	envelopes chan *v2.Envelope
}

func NewSpySetter() *SpySetter {
	return &SpySetter{
		envelopes: make(chan *v2.Envelope, 100),
	}
}

func (s *SpySetter) Set(e *v2.Envelope) {
	s.envelopes <- e
}
