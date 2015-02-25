package main_test

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	instrumentationtesthelpers "github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"

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

	Describe("Container Metrics", func() {
		var receivedChan chan []byte
		var dontKeepAliveChan chan bool
		var connectionDroppedChannel <-chan bool
		var ws *websocket.Conn

		var containerMetric *events.ContainerMetric

		BeforeEach(func() {
			connection, _ := net.Dial("udp", "127.0.0.1:3457")

			// Opening stream websocket to verify receipt of container metric
			waitChan := make(chan []byte)
			waitSocket, _, _ := AddWSSink(waitChan, "8083", "/apps/myApp/stream")

			containerMetric = factories.NewContainerMetric("myApp", 0, 1, 2, 3)
			envelope := MarshalEvent(containerMetric, "secret")
			_, err := connection.Write(envelope)
			Expect(err).NotTo(HaveOccurred())

			// Received something in the stream websocket so now we can listen for container metrics.
			// This ensures that we don't listen for containermetrics before it is processed.
			Eventually(waitChan).Should(Receive())
			waitSocket.Close()
		})

		AfterEach(func(done Done) {
			close(dontKeepAliveChan)
			ws.Close()
			Eventually(receivedChan).Should(BeClosed())
			close(done)
		})

		It("works from udp socket to websocket client", func() {
			receivedChan = make(chan []byte)
			ws, dontKeepAliveChan, connectionDroppedChannel = AddWSSink(receivedChan, "8083", "/apps/myApp/containermetrics")

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			var receivedEnvelope events.Envelope
			err := proto.Unmarshal(receivedMessageBytes, &receivedEnvelope)
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ContainerMetric))
			receivedMetric := receivedEnvelope.GetContainerMetric()
			Expect(receivedMetric).To(Equal(containerMetric))
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

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	var ws *websocket.Conn

	i := 0
	for {
		var err error
		ws, _, err = websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
		if err != nil {
			i++
			if i > 10 {
				fmt.Printf("Unable to connect to Server in 100ms, giving up.\n")
				return nil, nil, nil
			}
			<-time.After(10 * time.Millisecond)
			continue
		} else {
			break
		}

	}

	ws.SetPingHandler(func(message string) error {
		select {
		case <-dontKeepAliveChan:
			// do nothing
		default:
			ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})

		}
		return nil
	})

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				close(connectionDroppedChannel)
				close(receivedChan)
				return
			}
			receivedChan <- data
		}

	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}

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

func parseProtoBufMessageString(actual []byte) string {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(actual, &receivedEnvelope)
	if err != nil {
		Fail(err.Error())
	}
	return string(receivedEnvelope.GetLogMessage().GetMessage())
}
