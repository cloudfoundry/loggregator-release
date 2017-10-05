package doppler_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"net"
	"sync"
	"testing"

	"time"

	"code.cloudfoundry.org/loggregator/integration_tests/binaries"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/testservers"
	"code.cloudfoundry.org/workpool"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDoppler(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Integration Suite")
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

func dopplerEgressV1Client(addr string) (func(), plumbing.DopplerClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	out, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return func() {
		_ = out.Close()
	}, plumbing.NewDopplerClient(out)
}

func dopplerEgressV2Client(addr string) (func(), loggregator_v2.EgressClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	out, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return func() {
		_ = out.Close()
	}, loggregator_v2.NewEgressClient(out)
}

func dopplerIngressV1Client(addr string) (func(), plumbing.DopplerIngestor_PusherClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	client := plumbing.NewDopplerIngestorClient(conn)

	var (
		pusherClient plumbing.DopplerIngestor_PusherClient
		cancel       func()
	)
	f := func() func() {
		var (
			err error
			ctx context.Context
		)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		pusherClient, err = client.Pusher(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return cancel // when the cancel func escapes the linter is happy
	}
	Eventually(f).ShouldNot(BeNil())

	return func() {
		cancel()
		conn.Close()
	}, pusherClient
}

func dopplerIngressV2Client(addr string) (func(), loggregator_v2.DopplerIngress_SenderClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	client := loggregator_v2.NewDopplerIngressClient(conn)

	var (
		senderClient loggregator_v2.DopplerIngress_SenderClient
		cancel       func()
	)
	f := func() func() {
		var (
			err error
			ctx context.Context
		)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		senderClient, err = client.Sender(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return cancel // when the cancel func escapes the linter is happy
	}
	Eventually(f).ShouldNot(BeNil())

	return func() {
		cancel()
		conn.Close()
	}, senderClient
}

func sendAppLog(appID string, message string, client plumbing.DopplerIngestor_PusherClient) error {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

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
	envelope, _ := emitter.Wrap(event, "origin")

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

func etcdAdapter(url string) *etcdstoreadapter.ETCDStoreAdapter {
	pool, err := workpool.NewWorkPool(10)
	Expect(err).NotTo(HaveOccurred())
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: []string{url},
	}
	etcdAdapter, err := etcdstoreadapter.New(options, pool)
	Expect(err).ToNot(HaveOccurred())
	return etcdAdapter
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

func primePumpV2(ingressClient loggregator_v2.DopplerIngress_SenderClient, subscribeClient plumbing.Doppler_SubscribeClient) {
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
