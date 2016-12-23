package trafficcontroller_test

import (
	"integration_tests/trafficcontroller/fake_auth_server"
	"integration_tests/trafficcontroller/fake_doppler"
	"integration_tests/trafficcontroller/fake_uaa_server"
	"log"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/localip"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"

	"fmt"
	"net/http"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	APP_ID                            = "1234"
	AUTH_TOKEN                        = "bearer iAmAnAdmin"
	SUBSCRIPTION_ID                   = "firehose-subscription-1"
	TRAFFIC_CONTROLLER_DROPSONDE_PORT = 4566
)

var (
	trafficControllerExecPath string
	trafficControllerSession  *gexec.Session
	etcdRunner                *etcdstorerunner.ETCDClusterRunner
	etcdPort                  int
	localIPAddress            string
	fakeDoppler               *fake_doppler.FakeDoppler
	configFile                string
)

func TestIntegrationTest(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", log.LstdFlags))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Traffic Controller Integration Suite")
}

var _ = BeforeSuite(func() {
	killEtcdCmd := exec.Command("pkill", "etcd")
	killEtcdCmd.Run()
	setupEtcdAdapter()
	setupDopplerInEtcd()
	setupFakeAuthServer()
	setupFakeUaaServer()

	var err error
	trafficControllerExecPath, err = gexec.Build("trafficcontroller", "-race")
	Expect(err).ToNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()
})

var _ = BeforeEach(func() {
	configFile = "fixtures/trafficcontroller.json"
})

var _ = JustBeforeEach(func() {
	trafficControllerCommand := exec.Command(trafficControllerExecPath, "--config", configFile)

	var err error
	trafficControllerSession, err = gexec.Start(trafficControllerCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	// wait for TC
	trafficControllerDropsondeEndpoint := fmt.Sprintf("http://%s:%d", localIPAddress, 4566)
	Eventually(func() error {
		resp, err := http.Get(trafficControllerDropsondeEndpoint)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}).Should(Succeed())
})

var _ = AfterEach(func() {
	trafficControllerSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
	etcdRunner.Stop()
})

func setupEtcdAdapter() {
	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
}

func setupDopplerInEtcd() {
	adapter := etcdRunner.Adapter(nil)

	node := storeadapter.StoreNode{
		Key:   "/healthstatus/doppler/z1/doppler/0",
		Value: []byte("localhost"),
	}
	adapter.Create(node)

	node = storeadapter.StoreNode{
		Key:   "/doppler/meta/z1/doppler/0",
		Value: []byte(`{"version":1,"endpoints":["ws://localhost:1234"]}`),
	}
	adapter.Create(node)
}

var setupFakeAuthServer = func() {
	fakeAuthServer := &fake_auth_server.FakeAuthServer{ApiEndpoint: ":42123"}
	fakeAuthServer.Start()

	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":42123")
		return err
	}).ShouldNot(HaveOccurred())
}

var setupFakeUaaServer = func() {
	fakeUaaServer := &fake_uaa_server.FakeUaaHandler{}
	go http.ListenAndServe(":5678", fakeUaaServer)
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":5678/check_token")
		return err
	}).ShouldNot(HaveOccurred())
}

func makeDropsondeMessage(messageString string, appId string, currentTime int64) []byte {
	logMessage := &events.LogMessage{
		Message:        []byte(messageString),
		MessageType:    events.LogMessage_ERR.Enum(),
		Timestamp:      proto.Int64(currentTime),
		AppId:          proto.String(appId),
		SourceType:     proto.String("DOP"),
		SourceInstance: proto.String("SN"),
	}

	envelope := &events.Envelope{
		LogMessage: logMessage,
		Origin:     proto.String("doppler"),
		EventType:  events.Envelope_LogMessage.Enum(),
	}
	msg, _ := proto.Marshal(envelope)

	return msg
}
func makeContainerMetricMessage(appId string, instanceIndex int, cpu int, membytes int, diskbytes int, currentTime int64) []byte {
	containerMetric := &events.ContainerMetric{
		ApplicationId: proto.String(appId),
		InstanceIndex: proto.Int32(int32(instanceIndex)),
		CpuPercentage: proto.Float64(float64(cpu)),
		MemoryBytes:   proto.Uint64(uint64(membytes)),
		DiskBytes:     proto.Uint64(uint64(diskbytes)),
	}

	envelope := &events.Envelope{
		ContainerMetric: containerMetric,
		Origin:          proto.String("doppler"),
		EventType:       events.Envelope_ContainerMetric.Enum(),
		Timestamp:       proto.Int64(currentTime),
	}
	msg, _ := proto.Marshal(envelope)

	return msg
}

type RouterStart struct {
	Id                               string   `json:"id"`
	Hosts                            []string `json:"hosts"`
	MinimumRegisterIntervalInSeconds int      `json:"minimumRegisterIntervalInSeconds"`
}
