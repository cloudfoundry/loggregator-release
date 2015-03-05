package main_test

import (
	"net"
	"runtime"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	instrumentationtesthelpers "github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TODO: test error logging and metrics from unmarshaller stage
// messageRouter.Metrics.UnmarshalErrorsInParseEnvelopes++
//messageRouter.logger.Errorf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, envelopedLog)

var _ = Describe("Doppler Server", func() {

	Describe("ity's", func() {
		It("doesn't leak goroutines when sinks timeout", func() {
			time.Sleep(time.Duration(dopplerConfig.SinkInactivityTimeoutSeconds) * time.Second) // let existing application sinks timeout

			goroutineCount := runtime.NumGoroutine()

			connection, _ := net.Dial("udp", "127.0.0.1:3457")

			expectedMessageString := "Some Data"
			unmarshalledLogMessage := factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp-unique-for-goroutine-leak-test", "App")
			expectedMessage := MarshalEvent(unmarshalledLogMessage, "secret")

			_, err := connection.Write(expectedMessage)
			Expect(err).To(BeNil())
			Eventually(runtime.NumGoroutine).Should(Equal(goroutineCount + 2))
			time.Sleep(time.Duration(dopplerConfig.SinkInactivityTimeoutSeconds) * time.Second)
			Expect(runtime.NumGoroutine()).To(Equal(goroutineCount))
		})
	})

	Context("metric emission", func() {
		var getEmitter = func(name string) instrumentation.Instrumentable {
			for _, emitter := range dopplerInstance.Emitters() {
				context := emitter.Emit()
				if context.Name == name {
					return emitter
				}
			}
			return nil
		}

		It("emits metrics for the dropsonde message listener", func() {
			emitter := getEmitter("dropsondeListener")
			countBefore := instrumentationtesthelpers.MetricValue(emitter, "receivedMessageCount").(uint64)

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write([]byte{1, 2, 3})

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "receivedMessageCount", countBefore+1)
		})

		It("emits metrics for the dropsonde unmarshaller", func() {
			emitter := getEmitter("dropsondeUnmarshaller")
			countBefore := instrumentationtesthelpers.MetricValue(emitter, "heartbeatReceived").(uint64)

			connection, _ := net.Dial("udp", "127.0.0.1:3457")

			envelope := &events.Envelope{
				Origin:    proto.String("fake-origin-3"),
				EventType: events.Envelope_Heartbeat.Enum(),
				Heartbeat: factories.NewHeartbeat(1, 2, 3),
			}
			message, _ := proto.Marshal(envelope)
			signedMessage := signature.SignMessage(message, []byte("secret"))
			connection.Write(signedMessage)

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "heartbeatReceived", countBefore+1)
		})

		It("emits metrics for the dropsonde signature verifier", func() {
			emitter := getEmitter("signatureVerifier")
			countBefore := instrumentationtesthelpers.MetricValue(emitter, "missingSignatureErrors").(uint64)

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write([]byte{1, 2, 3})

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "missingSignatureErrors", countBefore+1)
		})
	})
})

func MarshalEvent(event events.Event, secret string) []byte {
	envelope, _ := emitter.Wrap(event, "origin")
	envelopeBytes := marshalProtoBuf(envelope)

	return signature.SignMessage(envelopeBytes, []byte(secret))
}

func marshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	if err != nil {
		Fail(err.Error())
	}

	return marshalledProtoBuf
}
