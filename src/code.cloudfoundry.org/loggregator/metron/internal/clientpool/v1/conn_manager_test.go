package v1_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	clientpool "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnManager", func() {
	var (
		connManager      *clientpool.ConnManager
		mockConnector    *mockConnector
		mockCloser       *mockCloser
		mockPusherClient *mockDopplerIngestor_PusherClient
	)

	BeforeEach(func() {
		mockConnector = newMockConnector()
		connManager = clientpool.NewConnManager(mockConnector, 5, time.Minute)
		mockCloser = newMockCloser()
		mockPusherClient = newMockDopplerIngestor_PusherClient()
	})

	Context("when a connection is able to be established", func() {
		BeforeEach(func() {
			mockConnector.ConnectOutput.Ret0 <- mockCloser
			mockConnector.ConnectOutput.Ret1 <- mockPusherClient
			mockConnector.ConnectOutput.Ret2 <- nil
		})

		Context("when Send() does not return an error", func() {
			BeforeEach(func() {
				close(mockPusherClient.SendOutput.Ret0)
			})

			It("sends the message down the connection", func() {
				msg := []byte("some-data")
				f := func() error {
					return connManager.Write(msg)
				}
				Eventually(f).Should(Succeed())

				Eventually(mockPusherClient.SendInput.Arg0).Should(Receive(Equal(
					&plumbing.EnvelopeData{
						Payload: msg,
					},
				)))
			})

			Describe("connection recycling", func() {
				BeforeEach(func() {
					close(mockCloser.CloseOutput.Ret0)
					mockConnector.ConnectOutput.Ret0 <- mockCloser
					mockConnector.ConnectOutput.Ret1 <- mockPusherClient
					mockConnector.ConnectOutput.Ret2 <- nil
				})

				It("recycles the connections after max writes and reconnects", func() {
					msg := []byte("some-data")
					f := func() int {
						connManager.Write(msg)
						return len(mockConnector.ConnectCalled)
					}
					Eventually(f).Should(Equal(2))

					Expect(len(mockCloser.CloseCalled)).ToNot(BeZero())
				})
			})
		})

		Context("when Send() returns an error", func() {
			BeforeEach(func() {
				mockPusherClient.SendOutput.Ret0 <- nil
				f := func() error {
					return connManager.Write([]byte("some-data"))
				}
				Eventually(f).Should(Succeed())

				mockPusherClient.SendOutput.Ret0 <- errors.New("some-error")
				mockCloser.CloseOutput.Ret0 <- nil
			})

			It("returns an error and closes the closer", func() {
				err := connManager.Write([]byte("some-data"))
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
				return connManager.Write([]byte("some-data"))
			}
			Consistently(f).Should(HaveOccurred())
		})
	})
})
