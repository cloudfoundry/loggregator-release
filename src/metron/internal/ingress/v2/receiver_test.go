package v2_test

import (
	"errors"
	"io"

	ingress "metron/internal/ingress/v2"
	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Receiver", func() {
	var (
		rx *ingress.Receiver

		spySender *SpySender
		spySetter *SpySetter
	)

	BeforeEach(func() {
		spySender = NewSpySender()
		spySetter = NewSpySetter()

		rx = ingress.NewReceiver(spySetter)
	})

	It("calls set on the data setter with the data", func() {
		e := &v2.Envelope{
			SourceId: "some-id",
		}

		spySender.recvResponses <- RecvResponse{
			envelope: e,
		}
		spySender.recvResponses <- RecvResponse{
			envelope: e,
		}
		spySender.recvResponses <- RecvResponse{
			err: io.EOF,
		}

		rx.Sender(spySender)

		Eventually(spySetter.envelopes).Should(Receive(Equal(e)))
		Eventually(spySetter.envelopes).Should(Receive(Equal(e)))
	})

	It("returns an error when receive fails", func() {
		spySender.recvResponses <- RecvResponse{
			err: errors.New("error occurred"),
		}

		err := rx.Sender(spySender)

		Expect(err).To(HaveOccurred())
	})
})

type RecvResponse struct {
	envelope *v2.Envelope
	err      error
}

type SpySender struct {
	v2.Ingress_SenderServer
	recvResponses chan RecvResponse
}

func NewSpySender() *SpySender {
	return &SpySender{
		recvResponses: make(chan RecvResponse, 100),
	}
}

func (s *SpySender) Recv() (*v2.Envelope, error) {
	resp := <-s.recvResponses

	return resp.envelope, resp.err
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
