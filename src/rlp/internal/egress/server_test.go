package egress_test

import (
	"fmt"
	"io"
	"rlp/internal/egress"

	"golang.org/x/net/context"

	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	var (
		mockSubscriber     *mockSubscriber
		mockReceiverServer *mockReceiverServer
		server             *egress.Server
		ctx                context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockReceiverServer = newMockReceiverServer()
		mockSubscriber = newMockSubscriber()
		server = egress.NewServer(mockSubscriber)

		mockReceiverServer.ContextOutput.Ret0 <- ctx
	})

	Describe("Receiver()", func() {
		Context("when subscriber does not return an error", func() {
			var (
				rx      func() (*v2.Envelope, error)
				dataOut chan *v2.Envelope
				errOut  chan error
			)

			BeforeEach(func() {
				dataOut = make(chan *v2.Envelope, 100)
				errOut = make(chan error, 100)
				rx = func() (*v2.Envelope, error) {
					return <-dataOut, <-errOut
				}

				close(mockSubscriber.SubscribeOutput.Err)
				mockSubscriber.SubscribeOutput.Rx <- rx
			})

			It("uses the request", func() {
				dataOut <- nil
				errOut <- io.EOF

				req := &v2.EgressRequest{Filter: &v2.Filter{SourceId: "some-id"}}
				err := server.Receiver(req, mockReceiverServer)
				Expect(err).To(Equal(io.EOF))

				Expect(mockSubscriber.SubscribeInput.Ctx).To(Receive(Equal(ctx)))
				Expect(mockSubscriber.SubscribeInput.Request).To(Receive(Equal(req)))
			})

			It("streams data from the subscriber until an error from the subscriber", func() {
				close(mockReceiverServer.SendOutput.Ret0)
				e := &v2.Envelope{Timestamp: 1}

				dataOut <- e
				errOut <- nil
				dataOut <- nil
				errOut <- fmt.Errorf("some-secret-error")

				err := server.Receiver(&v2.EgressRequest{}, mockReceiverServer)
				Expect(err).To(Equal(io.ErrUnexpectedEOF))

				Expect(mockReceiverServer.SendInput.Arg0).To(Receive(Equal(e)))
			})

			It("streams data from the subscriber until an error from the sender", func() {
				close(errOut)
				e := &v2.Envelope{Timestamp: 1}

				dataOut <- e
				dataOut <- nil
				mockReceiverServer.SendOutput.Ret0 <- fmt.Errorf("some-secret-error")

				err := server.Receiver(&v2.EgressRequest{}, mockReceiverServer)
				Expect(err).To(Equal(io.ErrUnexpectedEOF))

				Expect(mockReceiverServer.SendInput.Arg0).To(Receive(Equal(e)))
			})
		})

		Context("when subscriber returns an error", func() {
			BeforeEach(func() {
				close(mockSubscriber.SubscribeOutput.Rx)
				mockSubscriber.SubscribeOutput.Err <- fmt.Errorf("some-error")
			})

			It("returns an error", func() {
				err := server.Receiver(&v2.EgressRequest{Filter: &v2.Filter{SourceId: "some-id"}}, mockReceiverServer)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
