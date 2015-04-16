package main_test

import (
	doppler "doppler"

	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	instrumentationtesthelpers "github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Doppler Server", func() {
	var dopplerInstance *doppler.Doppler

	BeforeEach(func() {
		storeAdapter := doppler.NewStoreAdapter(dopplerConfig.EtcdUrls, dopplerConfig.EtcdMaxConcurrentRequests)
		dopplerInstance = doppler.New("127.0.0.1", dopplerConfig, loggertesthelper.Logger(), storeAdapter, "dropsondeOrigin")
		go dopplerInstance.Start()
		time.Sleep(10 * time.Millisecond)
	})

	AfterEach(func() {
		dopplerInstance.Stop()
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

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write([]byte{1, 2, 3})

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "receivedMessageCount", 1)
		})

		It("emits metrics for the dropsonde unmarshaller", func() {
			emitter := getEmitter("dropsondeUnmarshaller")

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write(createSignedMessageFromHeartbeatEnvelope())

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "heartbeatReceived", 1)
		})

		It("emits metrics for the dropsonde signature verifier", func() {
			emitter := getEmitter("signatureVerifier")

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write([]byte{1, 2, 3})

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "missingSignatureErrors", 1)
		})

		It("emits metrics for the message router", func() {
			emitter := getEmitter("httpServer")

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write(createSignedMessageFromHeartbeatEnvelope())

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "receivedMessages", 1)
		})

		It("emits metrics for the sink manager", func() {
			emitter := getEmitter("messageRouter")

			connection, _ := net.Dial("udp", "127.0.0.1:3457")
			connection.Write(createSignedMessageFromHeartbeatEnvelope()) // goes to appID "system"

			unmarshalledLogMessage := factories.NewLogMessage(events.LogMessage_OUT, "message", "myApp-unique-for-goroutine-leak-test", "App")
			connection.Write(MarshalEvent(unmarshalledLogMessage, "secret"))

			instrumentationtesthelpers.EventuallyExpectMetric(emitter, "numberOfDumpSinks", 2)
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

func createSignedMessageFromHeartbeatEnvelope() []byte {
	envelope := &events.Envelope{
		Origin:    proto.String("fake-origin-3"),
		EventType: events.Envelope_Heartbeat.Enum(),
		Heartbeat: factories.NewHeartbeat(1, 2, 3),
	}
	message, _ := proto.Marshal(envelope)
	return signature.SignMessage(message, []byte("secret"))
}
