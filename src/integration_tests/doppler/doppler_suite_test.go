package doppler_test

import (
	"encoding/binary"
	"os/exec"
	"testing"

	"doppler/config"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"

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

	streamRegisteredEvent   []byte
	firehoseRegisteredEvent []byte
	prefixedLogMessage      []byte
	logMessage              []byte
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

	streamRegisteredEvent, err = proto.Marshal(&events.CounterEvent{
		Name:  proto.String("stream-registered"),
		Delta: proto.Uint64(1),
	})
	Expect(err).ToNot(HaveOccurred())

	firehoseRegisteredEvent, err = proto.Marshal(&events.CounterEvent{
		Name:  proto.String("firehose-registered"),
		Delta: proto.Uint64(1),
	})
	Expect(err).ToNot(HaveOccurred())
	e := &events.Envelope{
		Origin:    proto.String("foo"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:     []byte("foo"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
		},
	}

	logMessage, err = proto.Marshal(e)
	Expect(err).ToNot(HaveOccurred())

	length := uint32(len(logMessage))
	prefixedLogMessage = append(make([]byte, 4), logMessage...)
	binary.LittleEndian.PutUint32(prefixedLogMessage, length)
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
