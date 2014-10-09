package integration_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/gunk/localip"
	"github.com/cloudfoundry/noaa/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"net/http"
	"os/exec"
	"testing"
	"trafficcontroller/integration_test/fake_auth_server"
	"trafficcontroller/integration_test/fake_doppler"
	"trafficcontroller/integration_test/fake_uaa_server"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IntegrationTest Suite")
}

var _ = BeforeSuite(func() {
	killEtcdCmd := exec.Command("pkill", "etcd")
	killEtcdCmd.Run()

	setupEtcdAdapter()
	setupDopplerInEtcd()
	setupFakeDoppler()
	setupFakeAuthServer()
	setupFakeUaaServer()

	gnatsdExec, err := gexec.Build("github.com/apcera/gnatsd")
	Expect(err).ToNot(HaveOccurred())
	gnatsdCommand = exec.Command(gnatsdExec, "-p", "4222")

	gexec.Start(gnatsdCommand, nil, nil)

	err = exec.Command("go", "get", "-d", "github.com/cloudfoundry/gorouter").Run()
	Expect(err).NotTo(HaveOccurred())

	routerExec, err := gexec.Build("github.com/cloudfoundry/gorouter")
	Expect(err).ToNot(HaveOccurred())
	routerCommand = exec.Command(routerExec)
	gexec.Start(routerCommand, nil, nil)

	pathToTrafficControllerExec, err := gexec.Build("trafficcontroller")
	Expect(err).ToNot(HaveOccurred())

	command = exec.Command(pathToTrafficControllerExec, "--config=fixtures/trafficcontroller.json", "--debug")
	gexec.Start(command, GinkgoWriter, GinkgoWriter)

	localIPAddress, _ = localip.LocalIP()

	// wait for servers to be up
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":1235")
		return err
	}).ShouldNot(HaveOccurred())
	<-fakeDoppler.TrafficControllerConnected

	Eventually(func() error {
		trafficControllerDropsondeEndpoint := fmt.Sprintf("http://%s:%d", localIPAddress, 4566)
		_, err := http.Get(trafficControllerDropsondeEndpoint)
		return err
	}).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	command.Process.Kill()
	gnatsdCommand.Process.Kill()
	routerCommand.Process.Kill()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()

	fakeDoppler.Stop()
})

var command *exec.Cmd
var gnatsdCommand *exec.Cmd
var routerCommand *exec.Cmd
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string
var fakeDoppler *fake_doppler.FakeDoppler

const APP_ID = "1234"
const AUTH_TOKEN = "bearer iAmAnAdmin"

var setupFakeDoppler = func() {
	fakeDoppler = &fake_doppler.FakeDoppler{ApiEndpoint: ":1235"}
	fakeDoppler.Start()
	<-fakeDoppler.Ready
}

func setupEtcdAdapter() {
	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
}

func setupDopplerInEtcd() {
	node := storeadapter.StoreNode{
		Key:   "/healthstatus/doppler/z1/doppler/0",
		Value: []byte("localhost"),
	}
	adapter := etcdRunner.Adapter()
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
