package egress_test

import (
	"fmt"
	"io"
	"metricemitter"
	"metricemitter/testhelper"
	"rlp/internal/egress"

	"golang.org/x/net/context"

	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	var (
		mockReceiver       *mockReceiver
		mockReceiverServer *mockReceiverServer
		server             *egress.Server
		ctx                context.Context
		metricClient       metricemitter.MetricClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockReceiverServer = newMockReceiverServer()
		mockReceiver = newMockReceiver()
		metricClient = testhelper.NewMetricClient()
		server = egress.NewServer(mockReceiver, metricClient)

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

				close(mockReceiver.ReceiveOutput.Err)
				mockReceiver.ReceiveOutput.Rx <- rx
			})

			It("returns an error for a request that has type filter but not a source ID", func() {
				req := &v2.EgressRequest{
					Filter: &v2.Filter{
						Message: &v2.Filter_Log{
							Log: &v2.LogFilter{},
						},
					},
				}
				err := server.Receiver(req, mockReceiverServer)

				Expect(err).To(HaveOccurred())
			})

			It("uses the request", func() {
				dataOut <- nil
				errOut <- io.EOF

				req := &v2.EgressRequest{Filter: &v2.Filter{SourceId: "some-id"}}
				err := server.Receiver(req, mockReceiverServer)
				Expect(err).To(Equal(io.EOF))

				Expect(mockReceiver.ReceiveInput.Ctx).To(Receive(Equal(ctx)))
				Expect(mockReceiver.ReceiveInput.Request).To(Receive(Equal(req)))
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
				close(mockReceiver.ReceiveOutput.Rx)
				mockReceiver.ReceiveOutput.Err <- fmt.Errorf("some-error")
			})

			It("returns an error", func() {
				err := server.Receiver(&v2.EgressRequest{Filter: &v2.Filter{SourceId: "some-id"}}, mockReceiverServer)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
