package legacymessage_test

import (
	"metron/legacymessage"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LegacyUnmarshaller", func() {
	var (
		inputChan    chan []byte
		outputChan   chan *logmessage.LogEnvelope
		runComplete  chan struct{}
		unmarshaller *legacymessage.Unmarshaller
	)

	Context("Unmarshal", func() {
		BeforeEach(func() {
			unmarshaller = legacymessage.NewUnmarshaller(loggertesthelper.Logger())
		})
		It("unmarshalls bytes", func() {
			input := &logmessage.LogEnvelope{
				RoutingKey: proto.String("fake-routing-key"),
				Signature:  []byte{1, 2, 3},
				LogMessage: &logmessage.LogMessage{
					Message:     []byte{4, 5, 6},
					MessageType: logmessage.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(123),
					AppId:       proto.String("fake-app-id"),
				},
			}
			message, _ := proto.Marshal(input)

			output, _ := unmarshaller.UnmarshalMessage(message)

			Expect(output).To(Equal(input))
		})

		It("handles bad input gracefully", func() {
			output, err := unmarshaller.UnmarshalMessage(make([]byte, 4))
			Expect(output).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Run", func() {

		BeforeEach(func() {
			inputChan = make(chan []byte, 10)
			outputChan = make(chan *logmessage.LogEnvelope, 10)
			runComplete = make(chan struct{})
			unmarshaller = legacymessage.NewUnmarshaller(loggertesthelper.Logger())

			go func() {
				unmarshaller.Run(inputChan, outputChan)
				close(runComplete)
			}()
		})

		AfterEach(func() {
			close(inputChan)
			Eventually(runComplete).Should(BeClosed())
		})

		It("unmarshals bytes on channel into envelopes", func() {
			envelope := &logmessage.LogEnvelope{
				RoutingKey: proto.String("fake-routing-key"),
				Signature:  []byte{1, 2, 3},
				LogMessage: &logmessage.LogMessage{
					Message:     []byte{4, 5, 6},
					MessageType: logmessage.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(123),
					AppId:       proto.String("fake-app-id"),
				},
			}
			message, _ := proto.Marshal(envelope)

			inputChan <- message
			outputEnvelope := <-outputChan
			Expect(outputEnvelope).To(Equal(envelope))
		})

		It("does not put an envelope on the output channel if there is unmarshal error", func() {
			inputChan <- []byte{1, 2, 3}
			Consistently(outputChan).ShouldNot(Receive())
		})
	})

	Context("metrics", func() {
		BeforeEach(func() {
			inputChan = make(chan []byte, 10)
			outputChan = make(chan *logmessage.LogEnvelope, 10)
			runComplete = make(chan struct{})
			unmarshaller = legacymessage.NewUnmarshaller(loggertesthelper.Logger())

			go func() {
				unmarshaller.Run(inputChan, outputChan)
				close(runComplete)
			}()
		})

		AfterEach(func() {
			close(inputChan)
			Eventually(runComplete).Should(BeClosed())
		})

		It("emits the correct metrics context", func() {
			Expect(unmarshaller.Emit().Name).To(Equal("legacyUnmarshaller"))
		})

		It("emits an unmarshal error counter", func() {
			inputChan <- []byte{1, 2, 3}
			testhelpers.EventuallyExpectMetric(unmarshaller, "unmarshalErrors", 1)
		})
	})
})
