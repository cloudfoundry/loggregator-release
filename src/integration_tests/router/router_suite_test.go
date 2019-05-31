package router_test

import (
	"bytes"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/integration_tests/binaries"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRounter(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rounter Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bp, errors := binaries.Build()
	for err := range errors {
		Expect(err).ToNot(HaveOccurred())
	}
	text, err := bp.Marshal()
	Expect(err).ToNot(HaveOccurred())
	return text
}, func(bpText []byte) {
	var bp binaries.BuildPaths
	err := bp.Unmarshal(bpText)
	Expect(err).ToNot(HaveOccurred())
	bp.SetEnv()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	binaries.Cleanup()
})

func buildLogMessage() []byte {
	e := &events.Envelope{
		Origin:    proto.String("foo"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("foo"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String("some-test-app-id"),
		},
	}
	b, err := proto.Marshal(e)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func sendAppLog(appID string, message string, client plumbing.DopplerIngestor_PusherClient) error {
	logMessage := NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

	return sendEvent(logMessage, client)
}

func sendEvent(event events.Event, client plumbing.DopplerIngestor_PusherClient) error {
	log := marshalEvent(event, "secret")

	err := client.Send(&plumbing.EnvelopeData{
		Payload: log,
	})
	return err
}

func marshalEvent(event events.Event, secret string) []byte {
	envelope, _ := Wrap(event, "origin")

	return marshalProtoBuf(envelope)
}

func marshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	Expect(err).NotTo(HaveOccurred())

	return marshalledProtoBuf
}

func decodeProtoBufEnvelope(actual []byte) *events.Envelope {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(actual, &receivedEnvelope)
	Expect(err).NotTo(HaveOccurred())
	return &receivedEnvelope
}

func decodeProtoBufLogMessage(actual []byte) *events.LogMessage {
	receivedEnvelope := decodeProtoBufEnvelope(actual)
	return receivedEnvelope.GetLogMessage()
}

func unmarshalMessage(messageBytes []byte) events.Envelope {
	var envelope events.Envelope
	err := proto.Unmarshal(messageBytes, &envelope)
	Expect(err).NotTo(HaveOccurred())
	return envelope
}

type tcpServer struct {
	mu    sync.Mutex
	_data bytes.Buffer

	listener net.Listener
	port     int
}

func (s *tcpServer) start() {
	go func() {
		defer GinkgoRecover()

		for {
			conn, err := s.listener.Accept()
			if err != nil {
				return
			}

			go func(conn net.Conn) {
				defer GinkgoRecover()

				for {
					data := make([]byte, 1024)
					_, err := conn.Read(data)
					if err != nil {
						return
					}

					s.mu.Lock()
					s._data.Write(data)
					s.mu.Unlock()
				}
			}(conn)
		}
	}()
}

func (s *tcpServer) readLine() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._data.ReadString('\n')
}

func (s *tcpServer) close() {
	s.listener.Close()
}

func startUnencryptedTCPServer(syslogDrainAddress string) (*tcpServer, error) {
	lis, err := net.Listen("tcp", syslogDrainAddress)
	if err != nil {
		return nil, err
	}

	server := &tcpServer{
		listener: lis,
		port:     lis.Addr().(*net.TCPAddr).Port,
	}
	server.start()

	return server, nil
}

func startEncryptedTCPServer(syslogDrainAddress string) (*tcpServer, error) {
	lis, err := net.Listen("tcp", syslogDrainAddress)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := plumbing.NewServerTLSConfig(
		testservers.Cert("localhost.crt"),
		testservers.Cert("localhost.key"),
	)
	Expect(err).NotTo(HaveOccurred())
	lis = tls.NewListener(lis, tlsConfig)

	server := &tcpServer{
		listener: lis,
		port:     lis.Addr().(*net.TCPAddr).Port,
	}
	server.start()

	return server, nil
}

func buildV1PrimerLogMessage() []byte {
	e := &events.Envelope{
		Origin:    proto.String("primer"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("primer"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String("some-test-app-id"),
		},
	}
	b, err := proto.Marshal(e)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func buildV2PrimerLogMessage() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: "primer",
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("primer"),
			},
		},
	}
}

func primePumpV1(ingressClient plumbing.DopplerIngestor_PusherClient, subscribeClient plumbing.Doppler_SubscribeClient) {
	message := buildV1PrimerLogMessage()

	// emit a bunch of primer messages
	go func() {
		for i := 0; i < 20; i++ {
			err := ingressClient.Send(&plumbing.EnvelopeData{
				Payload: message,
			})
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// wait for a single message to come through
	_, err := subscribeClient.Recv()
	Expect(err).ToNot(HaveOccurred())
}

func primePumpV2(ingressClient loggregator_v2.Ingress_SenderClient, subscribeClient plumbing.Doppler_SubscribeClient) {
	message := buildV2PrimerLogMessage()

	// emit a bunch of primer messages
	go func() {
		for i := 0; i < 20; i++ {
			err := ingressClient.Send(message)
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// wait for a single message to come through
	_, err := subscribeClient.Recv()
	Expect(err).ToNot(HaveOccurred())
}

var ErrorMissingOrigin = errors.New("Event not emitted due to missing origin information")
var ErrorUnknownEventType = errors.New("Cannot create envelope for unknown event type")

func Wrap(event events.Event, origin string) (*events.Envelope, error) {
	if origin == "" {
		return nil, ErrorMissingOrigin
	}

	envelope := &events.Envelope{Origin: proto.String(origin), Timestamp: proto.Int64(time.Now().UnixNano())}

	switch event := event.(type) {
	case *events.HttpStartStop:
		envelope.EventType = events.Envelope_HttpStartStop.Enum()
		envelope.HttpStartStop = event
	case *events.ValueMetric:
		envelope.EventType = events.Envelope_ValueMetric.Enum()
		envelope.ValueMetric = event
	case *events.CounterEvent:
		envelope.EventType = events.Envelope_CounterEvent.Enum()
		envelope.CounterEvent = event
	case *events.LogMessage:
		envelope.EventType = events.Envelope_LogMessage.Enum()
		envelope.LogMessage = event
	case *events.ContainerMetric:
		envelope.EventType = events.Envelope_ContainerMetric.Enum()
		envelope.ContainerMetric = event
	default:
		return nil, ErrorUnknownEventType
	}

	return envelope, nil
}

func NewLogMessage(messageType events.LogMessage_MessageType, messageString, appId, sourceType string) *events.LogMessage {
	currentTime := time.Now()

	logMessage := &events.LogMessage{
		Message:     []byte(messageString),
		AppId:       &appId,
		MessageType: &messageType,
		SourceType:  proto.String(sourceType),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}
