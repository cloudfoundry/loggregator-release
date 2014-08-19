package unmarshaller_test

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinkserver/unmarshaller"
)

var _ = Describe("LogMessageUnmarshaller", func() {

	var inChan chan []byte
	var secret string

	BeforeEach(func() {
		inChan = make(chan []byte)
		secret = "mySecretEncryptionKey"
	})

	Describe("New", func() {
		It("should return the unmarshaller stage and the open output channel", func() {
			anUnmarshaller, outChan := unmarshaller.NewLogMessageUnmarshaller(secret, inChan)
			Expect(anUnmarshaller).ToNot(BeZero())
			Expect(outChan).ToNot(BeClosed())
		})
	})

	Context("With an unmarshaller", func() {
		var anUnmarshaller *unmarshaller.LogMessageUnmarshaller
		var outChan <-chan *logmessage.Message
		var errorChan chan error

		BeforeEach(func() {
			errorChan = make(chan error)
			anUnmarshaller, outChan = unmarshaller.NewLogMessageUnmarshaller(secret, inChan)
			go anUnmarshaller.Start(errorChan)
		})

		Describe("Start", func() {
			It("should read byte from the inChan", func(done Done) {
				inChan <- []byte{1, 2, 3, 4}
				Eventually(inChan).Should(BeEmpty())
				close(done)
			})

			It("should send messages out on the outChan", func(done Done) {
				expectedMessage := "Hi"
				msgToSend := GenerateLogMessage(expectedMessage, "appID", logmessage.LogMessage_OUT, "App", "0")
				msgWithEnvolopeBytes := MarshalledLogEnvelope(msgToSend, secret)

				inChan <- msgWithEnvolopeBytes
				msg := <-outChan
				Expect(string(msg.GetLogMessage().GetMessage())).To(Equal(expectedMessage))
				close(done)
			})

			It("should send errors is the bytes are unmarshallable", func(done Done) {
				inChan <- []byte{1, 2, 3, 4}
				Expect(outChan).To(BeEmpty())
				error := <-errorChan
				Expect(error.Error()).To(ContainSubstring("illegal tag 0"))
				close(done)
			})

			Context("when input channel is closed", func() {
				BeforeEach(func() {
					close(inChan)
				})

				It("should close the output channel", func(done Done) {
					Expect(outChan).To(BeClosed())
					close(done)
				})
			})
		})
	})

})
