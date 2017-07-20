package doppler_test

import (
	"crypto/sha1"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"testing"

	"time"

	"code.cloudfoundry.org/loggregator/doppler/app"
	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestDoppler(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Integration Suite")
}

var (
	dopplerSession *gexec.Session
	localIPAddress string
	etcdRunner     *etcdstorerunner.ETCDClusterRunner
	etcdAdapter    storeadapter.StoreAdapter

	pathToDopplerExec    string
	pathToHTTPEchoServer string
	pathToTCPEchoServer  string
	pathToConfigFile     = "fixtures/doppler.json"
)

var _ = BeforeSuite(func() {
	var err error
	etcdPort := 5555
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	pathToDopplerExec, err = gexec.Build("code.cloudfoundry.org/loggregator/doppler", "-race")
	Expect(err).NotTo(HaveOccurred())

	pathToHTTPEchoServer, err = gexec.Build("tools/echo/cmd/http_server")
	Expect(err).NotTo(HaveOccurred())

	pathToTCPEchoServer, err = gexec.Build("tools/echo/cmd/tcp_server")
	Expect(err).NotTo(HaveOccurred())
})

var _ = JustBeforeEach(func() {
	var err error

	etcdRunner.Reset()

	command := exec.Command(pathToDopplerExec, "--config="+pathToConfigFile)
	dopplerSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	dopplerStartedFn := func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", 6790))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
	Eventually(dopplerStartedFn, 3).Should(BeTrue())
	localIPAddress = "127.0.0.1"
	Eventually(func() error {
		_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}, time.Second+app.HeartbeatInterval).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
	dopplerSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()

	if etcdRunner != nil {
		etcdRunner.Stop()
	}

	gexec.CleanupBuildArtifacts()
})

func buildLogMessage() []byte {
	e := &events.Envelope{
		Origin:    proto.String("foo"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("foo"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String("test-app"),
		},
	}
	b, err := proto.Marshal(e)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func buildContainerMetric() []byte {
	e := &events.Envelope{
		Origin:    proto.String("foo"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("test-app"),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(12.2),
			MemoryBytes:   proto.Uint64(1234),
			DiskBytes:     proto.Uint64(4321),
		},
	}
	b, err := e.Marshal()
	Expect(err).ToNot(HaveOccurred())
	return b
}

func connectToGRPC(conf *app.Config) (*grpc.ClientConn, plumbing.DopplerClient) {
	tlsCfg, err := plumbing.NewClientMutualTLSConfig(conf.GRPC.CertFile, conf.GRPC.KeyFile, conf.GRPC.CAFile, "doppler")
	Expect(err).ToNot(HaveOccurred())
	creds := credentials.NewTLS(tlsCfg)
	out, err := grpc.Dial(localIPAddress+":5678", grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return out, plumbing.NewDopplerClient(out)
}

func fetchDopplerConfig(path string) *app.Config {
	conf, err := app.ParseConfig(path)
	Expect(err).ToNot(HaveOccurred())
	return conf
}

func SendAppLog(appID string, message string, connection net.Conn) error {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

	return SendEvent(logMessage, connection)
}

func SendEvent(event events.Event, connection net.Conn) error {
	signedEnvelope := MarshalEvent(event, "secret")

	_, err := connection.Write(signedEnvelope)
	return err
}

func MarshalEvent(event events.Event, secret string) []byte {
	envelope, _ := emitter.Wrap(event, "origin")
	envelopeBytes := MarshalProtoBuf(envelope)

	return signature.SignMessage(envelopeBytes, []byte(secret))
}

func MarshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	Expect(err).NotTo(HaveOccurred())

	return marshalledProtoBuf
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, <-chan struct{}) {
	connectionDroppedChannel := make(chan struct{}, 1)

	var ws *websocket.Conn

	ip := "127.0.0.1"
	fullURL := "ws://" + ip + ":" + port + path

	Eventually(func() error {
		var err error
		ws, _, err = websocket.DefaultDialer.Dial(fullURL, http.Header{})
		return err
	}, 5, 1).ShouldNot(HaveOccurred(), fmt.Sprintf("Unable to connect to server at %s.", fullURL))

	ws.SetPingHandler(func(message string) error {
		ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})
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

	return ws, connectionDroppedChannel
}

func DecodeProtoBufEnvelope(actual []byte) *events.Envelope {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(actual, &receivedEnvelope)
	Expect(err).NotTo(HaveOccurred())
	return &receivedEnvelope
}

func DecodeProtoBufLogMessage(actual []byte) *events.LogMessage {
	receivedEnvelope := DecodeProtoBufEnvelope(actual)
	return receivedEnvelope.GetLogMessage()
}

func DecodeProtoBufCounterEvent(actual []byte) *events.CounterEvent {
	receivedEnvelope := DecodeProtoBufEnvelope(actual)
	return receivedEnvelope.GetCounterEvent()
}

func AppKey(appID string) string {
	return fmt.Sprintf("/loggregator/v2/services/%s", appID)
}

func DrainKey(appID string, drainURL string) string {
	hash := sha1.Sum([]byte(drainURL))
	return fmt.Sprintf("%s/%x", AppKey(appID), hash)
}

func AddETCDNode(etcdAdapter storeadapter.StoreAdapter, key string, value string) {
	node := storeadapter.StoreNode{
		Key:   key,
		Value: []byte(value),
		TTL:   uint64(20),
	}
	etcdAdapter.Create(node)
	recvNode, err := etcdAdapter.Get(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(recvNode.Value)).To(Equal(value))
}

func UnmarshalMessage(messageBytes []byte) events.Envelope {
	var envelope events.Envelope
	err := proto.Unmarshal(messageBytes, &envelope)
	Expect(err).NotTo(HaveOccurred())
	return envelope
}

func StartHTTPSServer(pathToHTTPEchoServer string) *gexec.Session {
	command := exec.Command(pathToHTTPEchoServer, "-cert", "../fixtures/server.crt", "-key", "../fixtures/server.key")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

func StartUnencryptedTCPServer(pathToTCPEchoServer string, syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}

func StartEncryptedTCPServer(pathToTCPEchoServer string, syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "../fixtures/server.crt", "-key", "../fixtures/server.key")
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}

func DialTLS(address, cert, key, ca string) (*tls.Conn, error) {
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		"../fixtures/client.crt",
		"../fixtures/client.key",
		"../fixtures/loggregator-ca.crt",
		"doppler",
	)
	Expect(err).NotTo(HaveOccurred())
	return tls.Dial("tcp", address, tlsConfig)
}

func SendAppLogTCP(appID string, message string, connection net.Conn) error {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

	return SendEventTCP(logMessage, connection)
}

func SendEventTCP(event events.Event, conn net.Conn) error {
	envelope, err := emitter.Wrap(event, "origin")
	Expect(err).NotTo(HaveOccurred())

	bytes, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}

	err = binary.Write(conn, binary.LittleEndian, uint32(len(bytes)))
	if err != nil {
		return err
	}

	_, err = conn.Write(bytes)
	return err
}
