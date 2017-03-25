package v2_test

import (
	"errors"
	"io"
	"sync"
	"time"

	clientpool "metron/internal/clientpool/v2"
	plumbing "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type SpyConnector struct {
	mu      sync.Mutex
	closer  io.Closer
	client  plumbing.DopplerIngress_SenderClient
	err     error
	called_ int
}

func (s *SpyConnector) Connect() (io.Closer, plumbing.DopplerIngress_SenderClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called_++
	return s.closer, s.client, s.err
}

func (s *SpyConnector) String() string {
	return ""
}

func (s *SpyConnector) called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called_
}

type SpyClient struct {
	envelope *plumbing.Envelope
	err      error
	plumbing.DopplerIngress_SenderClient
}

func (s *SpyClient) Send(e *plumbing.Envelope) error {
	s.envelope = e
	return s.err
}

type SpyCloser struct {
	called int
}

func (s *SpyCloser) Close() error {
	s.called++
	return nil
}

var _ = Describe("ConnManager", func() {
	var (
		connManager  *clientpool.ConnManager
		closer       *SpyCloser
		senderClient *SpyClient
		connector    *SpyConnector
	)

	Context("when a connection is able to be established", func() {
		BeforeEach(func() {
			senderClient = &SpyClient{}
			closer = &SpyCloser{}
			connector = &SpyConnector{
				closer: closer,
				client: senderClient,
			}
			connManager = clientpool.NewConnManager(connector, 5, time.Millisecond)
		})

		It("sends the message down the connection", func() {
			e := &plumbing.Envelope{SourceId: "some-uuid"}
			f := func() error {
				return connManager.Write(e)
			}
			Eventually(f).Should(Succeed())
			Expect(senderClient.envelope).To(Equal(e))
		})

		It("recycles the connections after max writes", func() {
			e := &plumbing.Envelope{SourceId: "some-uuid"}
			f := func() int {
				connManager.Write(e)
				return connector.called()
			}
			Eventually(f).Should(Equal(2))
			Expect(closer.called).ToNot(BeZero())
		})

		Context("when Send() returns an error", func() {
			BeforeEach(func() {
				f := func() error {
					return connManager.Write(&plumbing.Envelope{})
				}
				Eventually(f).Should(Succeed())
			})

			It("returns an error and closes the closer", func() {
				expectedErr := errors.New("It is the error")
				senderClient.err = expectedErr

				actualErr := connManager.Write(&plumbing.Envelope{SourceId: "some-uuid"})
				Expect(actualErr).To(Equal(expectedErr))
				Expect(closer.called).To(Equal(1))
			})
		})
	})

	Context("when a connection is not able to be established", func() {
		BeforeEach(func() {
			connector = &SpyConnector{
				err: errors.New("an error"),
			}
			connManager = clientpool.NewConnManager(connector, 5, time.Millisecond)
		})

		It("always returns an error", func() {
			f := func() error {
				return connManager.Write(&plumbing.Envelope{})
			}
			Consistently(f).Should(HaveOccurred())
		})
	})
})
