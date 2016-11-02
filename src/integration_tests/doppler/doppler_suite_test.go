package doppler_test

import (
	"encoding/binary"
	"os/exec"
	"plumbing"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"doppler/config"
	"time"

	"code.cloudfoundry.org/localip"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func TestDoppler(t *testing.T) {
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

	prefixedLogMessage    []byte
	logMessage            []byte
	prefixedPrimerMessage []byte
	primerMessage         []byte

	prefixedContainerMetric []byte
	containerMetric         []byte
)

var _ = BeforeSuite(func() {
	var err error
	etcdPort := 5555
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	pathToDopplerExec, err = gexec.Build("doppler", "-race")
	Expect(err).NotTo(HaveOccurred())

	pathToHTTPEchoServer, err = gexec.Build("tools/httpechoserver")
	Expect(err).NotTo(HaveOccurred())

	pathToTCPEchoServer, err = gexec.Build("tools/tcpechoserver")
	Expect(err).NotTo(HaveOccurred())

	logMessage = buildLogMessage()
	prefixedLogMessage = prefixMessage(logMessage)
	primerMessage = buildPrimerMessage()
	prefixedPrimerMessage = prefixMessage(primerMessage)

	containerMetric = buildContainerMetric()
	prefixedContainerMetric = prefixMessage(containerMetric)
})

var _ = BeforeEach(func() {
	var err error

	etcdRunner.Reset()

	command := exec.Command(pathToDopplerExec, "--config=fixtures/doppler.json", "--debug")
	dopplerSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(dopplerSession.Out, 3).Should(gbytes.Say("Startup: doppler server started"))
	localIPAddress, _ = localip.LocalIP()
	Eventually(func() error {
		_, err := etcdAdapter.Get("healthstatus/doppler/z1/doppler_z1/0")
		return err
	}, time.Second+config.HeartbeatInterval).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
	dopplerSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	etcdAdapter.Disconnect()
	etcdRunner.Stop()
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

func buildPrimerMessage() []byte {
	e := &events.Envelope{
		Origin:    proto.String("prime-message"),
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
	b, err := proto.Marshal(e)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func prefixMessage(msg []byte) []byte {
	length := uint32(len(msg))
	b := append(make([]byte, 4), msg...)
	binary.LittleEndian.PutUint32(b, length)
	return b
}

func connectToGRPC(conf *config.Config) (*grpc.ClientConn, plumbing.DopplerClient) {
	tlsCfg, err := plumbing.NewTLSConfig(conf.GRPC.CertFile, conf.GRPC.KeyFile, conf.GRPC.CAFile, "doppler")
	Expect(err).ToNot(HaveOccurred())
	creds := credentials.NewTLS(tlsCfg)
	out, err := grpc.Dial(localIPAddress+":5678", grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return out, plumbing.NewDopplerClient(out)
}

func fetchDopplerConfig(path string) *config.Config {
	conf, err := config.ParseConfig(path)
	Expect(err).ToNot(HaveOccurred())
	return conf
}
