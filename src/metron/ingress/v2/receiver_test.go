package v2_test

import (
	"errors"
	"io"
	ingress "metron/ingress/v2"
	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Receiver", func() {
	var (
		rx *ingress.Receiver

		mockDataSetter *mockDataSetter
		mockSender     *mockSender
	)

	BeforeEach(func() {
		mockSender = newMockSender()
		mockDataSetter = newMockDataSetter()

		rx = ingress.NewReceiver(mockDataSetter)
	})

	It("calls set on the data setter with the data", func() {
		e := &v2.Envelope{
			SourceId: "some-id",
		}
		mockSender.RecvOutput.Ret0 <- e
		mockSender.RecvOutput.Ret1 <- nil
		mockSender.RecvOutput.Ret0 <- e
		mockSender.RecvOutput.Ret1 <- nil
		mockSender.RecvOutput.Ret0 <- nil
		mockSender.RecvOutput.Ret1 <- io.EOF

		rx.Sender(mockSender)

		Eventually(mockDataSetter.SetInput.E).Should(Receive(Equal(e)))
		Eventually(mockDataSetter.SetInput.E).Should(Receive(Equal(e)))
	})

	It("returns an error when receive fails", func() {
		close(mockSender.RecvOutput.Ret0)
		mockSender.RecvOutput.Ret1 <- errors.New("error occurred")

		err := rx.Sender(mockSender)

		Expect(err).To(HaveOccurred())
	})
})
