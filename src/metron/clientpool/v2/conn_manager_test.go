package v2_test

import (
	"errors"
	clientpool "metron/clientpool/v2"
	plumbing "plumbing/v2"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnManager", func() {
	var (
		connManager      *clientpool.ConnManager
		mockConnector    *mockV2Connector
		mockCloser       *mockCloser
		mockSenderClient *mockDopplerIngress_SenderClient
	)

	BeforeEach(func() {
		mockConnector = newMockV2Connector()
		connManager = clientpool.NewConnManager(mockConnector, 5)
		mockCloser = newMockCloser()
		mockSenderClient = newMockDopplerIngress_SenderClient()
	})

	Context("when a connection is able to be established", func() {
		BeforeEach(func() {
			mockConnector.ConnectOutput.Ret0 <- mockCloser
			mockConnector.ConnectOutput.Ret1 <- mockSenderClient
			mockConnector.ConnectOutput.Ret2 <- nil
		})

		Context("when Send() does not return an error", func() {
			BeforeEach(func() {
				close(mockSenderClient.SendOutput.Ret0)
			})

			It("sends the message down the connection", func() {
				e := &plumbing.Envelope{SourceId: "some-uuid"}
				f := func() error {
					return connManager.Write(e)
				}
				Eventually(f).Should(Succeed())

				Eventually(mockSenderClient.SendInput.Arg0).Should(Receive(Equal(
					&plumbing.Envelope{SourceId: "some-uuid"},
				)))
			})

			Describe("connection recycling", func() {
				BeforeEach(func() {
					close(mockCloser.CloseOutput.Ret0)
					mockConnector.ConnectOutput.Ret0 <- mockCloser
					mockConnector.ConnectOutput.Ret1 <- mockSenderClient
					mockConnector.ConnectOutput.Ret2 <- nil
				})

				It("recycles the connections after max writes", func() {
					e := &plumbing.Envelope{SourceId: "some-uuid"}
					f := func() int {
						connManager.Write(e)
						return len(mockConnector.ConnectCalled)
					}
					Eventually(f).Should(Equal(2))

					Expect(len(mockCloser.CloseCalled)).ToNot(BeZero())
				})
			})
		})

		Context("when Send() returns an error", func() {
			BeforeEach(func() {
				mockSenderClient.SendOutput.Ret0 <- nil
				f := func() error {
					return connManager.Write(&plumbing.Envelope{SourceId: "some-uuid"})
				}
				Eventually(f).Should(Succeed())

				mockSenderClient.SendOutput.Ret0 <- errors.New("some-error")
				mockCloser.CloseOutput.Ret0 <- nil
			})

			It("returns an error and closes the closer", func() {
				err := connManager.Write(&plumbing.Envelope{SourceId: "some-uuid"})
				Expect(err).To(HaveOccurred())
				Expect(mockCloser.CloseCalled).To(HaveLen(1))
			})
		})
	})

	Context("when a connection is not able to be established", func() {
		BeforeEach(func() {
			close(mockConnector.ConnectOutput.Ret0)
			close(mockConnector.ConnectOutput.Ret1)
			testhelpers.AlwaysReturn(mockConnector.ConnectOutput.Ret2, errors.New("some-error"))
		})

		It("always returns an error", func() {
			f := func() error {
				return connManager.Write(&plumbing.Envelope{SourceId: "some-uuid"})
			}
			Consistently(f).Should(HaveOccurred())
		})
	})
})
