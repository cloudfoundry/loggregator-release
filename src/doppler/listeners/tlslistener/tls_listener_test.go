package tlslistener_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crypto/tls"
	"doppler/listeners/agentlistener"
	"doppler/listeners/tlslistener"
	"encoding/gob"
	"fmt"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"
	"github.com/nu7hatch/gouuid"
	"net"
	"net/http"
	"time"
)

const address = "127.0.0.1:4567"

var _ = Describe("Tcplistener", func() {

	Context("dropsonde metric emission", func() {
		var envelopeChan chan *events.Envelope
		var tlsListener agentlistener.Listener

		BeforeEach(func() {
			envelopeChan = make(chan *events.Envelope)
			tlsListener = tlslistener.New(address, config, envelopeChan, loggertesthelper.Logger())
			go tlsListener.Start()
		})

		AfterEach(func() {
			tlsListener.Stop()
		})

		It("sends all types of messages as a gob", func() {
			for name, eventType := range events.Envelope_EventType_value {
				envelope := createEnvelope(events.Envelope_EventType(eventType))
				conn := openTLSConnection()

				err := writeGob(conn, envelope)
				Expect(err).ToNot(HaveOccurred())

				Eventually(envelopeChan).Should(Receive(Equal(envelope)), fmt.Sprintf("did not receive expected event: %s", name))
				conn.Close()
			}
		})

		It("sends all types of messages over multiple connections", func() {
			for _, eventType := range events.Envelope_EventType_value {
				envelope1 := createEnvelope(events.Envelope_EventType(eventType))
				conn1 := openTLSConnection()

				envelope2 := createEnvelope(events.Envelope_EventType(eventType))
				conn2 := openTLSConnection()

				err := writeGob(conn1, envelope1)
				Expect(err).ToNot(HaveOccurred())
				err = writeGob(conn2, envelope2)
				Expect(err).ToNot(HaveOccurred())

				envelopes := readMessages(envelopeChan, 2)
				Expect(envelopes).To(ContainElement(envelope1))
				Expect(envelopes).To(ContainElement(envelope2))

				conn1.Close()
				conn2.Close()
			}
		})
	})

	Context("Start Stop", func() {
		var envelopeChan chan *events.Envelope
		var tlsListener agentlistener.Listener

		BeforeEach(func() {
			envelopeChan = make(chan *events.Envelope)
			tlsListener = tlslistener.New(address, config, envelopeChan, loggertesthelper.Logger())
		})

		It("fails to send message after listener has been stopped", func() {
			go tlsListener.Start()
			logMessage := factories.NewLogMessage(events.LogMessage_OUT, "some message", "appId", "source")
			envelope, _ := emitter.Wrap(logMessage, "origin")
			conn := openTLSConnection()

			err := writeGob(conn, envelope)
			Expect(err).ToNot(HaveOccurred())

			tlsListener.Stop()

			err = writeGob(conn, envelope)
			Expect(err).To(HaveOccurred())
		})

		It("can start again after being stopped", func() {
			//defer close(done)
			go tlsListener.Start()
			openTLSConnection()
			tlsListener.Stop()

			start := func() {
				go tlsListener.Start()
				openTLSConnection()
			}
			Expect(start).ShouldNot(Panic())
			tlsListener.Stop()
		})
	})
})

func readMessages(envelopeChan chan *events.Envelope, n int) []*events.Envelope {
	var envelopes []*events.Envelope
	for i := 0; i < n; i++ {
		envelope := <-envelopeChan
		envelopes = append(envelopes, envelope)
	}
	return envelopes
}

func openTLSConnection() net.Conn {

	var conn net.Conn
	var err error
	Eventually(func() error {
		conn, err = tls.Dial("tcp", address, &tls.Config{
			InsecureSkipVerify: true,
		})
		return err
	}).ShouldNot(HaveOccurred())

	return conn
}

func writeGob(conn net.Conn, envelope *events.Envelope) error {
	encoder := gob.NewEncoder(conn)
	return encoder.Encode(envelope)
}

func createEnvelope(eventType events.Envelope_EventType) *events.Envelope {

	envelope := &events.Envelope{Origin: proto.String("origin"), Timestamp: proto.Int64(time.Now().UnixNano())}

	switch eventType {
	case events.Envelope_HttpStart:
		req, _ := http.NewRequest("GET", "http://www.example.com", nil)
		req.RemoteAddr = "www.example.com"
		req.Header.Add("User-Agent", "user-agent")
		uuid, _ := uuid.NewV4()
		envelope.HttpStart = factories.NewHttpStart(req, events.PeerType_Client, uuid)
	case events.Envelope_HttpStop:
		req, _ := http.NewRequest("GET", "http://www.example.com", nil)
		uuid, _ := uuid.NewV4()
		envelope.HttpStop = factories.NewHttpStop(req, http.StatusOK, 128, events.PeerType_Client, uuid)
	case events.Envelope_HttpStartStop:
		req, _ := http.NewRequest("GET", "http://www.example.com", nil)
		req.RemoteAddr = "www.example.com"
		req.Header.Add("User-Agent", "user-agent")
		uuid, _ := uuid.NewV4()
		envelope.HttpStartStop = factories.NewHttpStartStop(req, http.StatusOK, 128, events.PeerType_Client, uuid)
	case events.Envelope_ValueMetric:
		envelope.ValueMetric = factories.NewValueMetric("some-value-metric", 123, "km")
	case events.Envelope_CounterEvent:
		envelope.CounterEvent = factories.NewCounterEvent("some-counter-event", 123)
	case events.Envelope_LogMessage:
		envelope.LogMessage = factories.NewLogMessage(events.LogMessage_OUT, "some message", "appId", "source")
	case events.Envelope_ContainerMetric:
		envelope.ContainerMetric = factories.NewContainerMetric("appID", 123, 1, 5, 5)
	case events.Envelope_Error:
		envelope.Error = factories.NewError("source", 123, "message")
	default:
		fmt.Printf("Unknown event %v\n", eventType)
		return nil
	}

	return envelope
}
