package integration_test

import (
	"integration_tests/trafficcontroller/fake_auth_server"
	"integration_tests/trafficcontroller/fake_doppler"
	"integration_tests/trafficcontroller/fake_uaa_server"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"

	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestIntegrationTest(t *testing.T) {
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

	gnatsdExec, err := gexec.Build("github.com/apcera/gnatsd")
	Expect(err).ToNot(HaveOccurred())
	gnatsdCommand := exec.Command(gnatsdExec, "-p", "4222")

	gnatsdSession, _ = gexec.Start(gnatsdCommand, nil, nil)

	StartFakeRouter()

	pathToTrafficControllerExec, err := gexec.Build("trafficcontroller", "-race")
	Expect(err).ToNot(HaveOccurred())

	tcCommand := exec.Command(pathToTrafficControllerExec, "--config=fixtures/trafficcontroller.json", "--debug")
	trafficControllerSession, err = gexec.Start(tcCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	// wait for TC
	trafficControllerDropsondeEndpoint := fmt.Sprintf("http://%s:%d", localIPAddress, 4566)
	Eventually(func() error {
		resp, err := http.Get(trafficControllerDropsondeEndpoint)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	trafficControllerSession.Kill().Wait()
	gnatsdSession.Kill().Wait()
	gexec.CleanupBuildArtifacts()
	etcdRunner.Stop()
})

var (
	trafficControllerSession *gexec.Session
	gnatsdSession            *gexec.Session
	etcdRunner               *etcdstorerunner.ETCDClusterRunner
	etcdPort                 int
	localIPAddress           string
	fakeDoppler              *fake_doppler.FakeDoppler
)

const APP_ID = "1234"
const AUTH_TOKEN = "bearer iAmAnAdmin"
const SUBSCRIPTION_ID = "firehose-subscription-1"

func setupEtcdAdapter() {
	etcdPort = 4001
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
}

func setupDopplerInEtcd() {
	node := storeadapter.StoreNode{
		Key:   "/healthstatus/doppler/z1/doppler/0",
		Value: []byte("localhost"),
	}
	adapter := etcdRunner.Adapter(nil)
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
func makeContainerMetricMessage(appId string, instanceIndex int32, cpu float64, currentTime int64) []byte {
	containerMetric := &events.ContainerMetric{
		ApplicationId: proto.String(appId),
		InstanceIndex: proto.Int32(instanceIndex),
		CpuPercentage: proto.Float64(cpu),
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

func StartFakeRouter() {

	var startMessage = func() []byte {
		d := RouterStart{
			MinimumRegisterIntervalInSeconds: 20,
		}

		value, _ := json.Marshal(d)
		return value
	}

	natsMembers := make([]string, 1)
	uri := url.URL{
		Scheme: "nats",
		User:   url.UserPassword("", ""),
		Host:   "127.0.0.1:4222",
	}
	natsMembers = append(natsMembers, uri.String())

	var natsClient yagnats.NATSConn
	var err error
	for i := 0; i < 10; i++ {
		natsClient, err = yagnats.Connect(natsMembers)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	Expect(err).ToNot(HaveOccurred())

	natsClient.Subscribe("router.register", func(msg *nats.Msg) {
	})
	natsClient.Subscribe("router.greet", func(msg *nats.Msg) {
		natsClient.Publish(msg.Reply, startMessage())
	})

	natsClient.Publish("router.start", startMessage())
}

type RouterStart struct {
	Id                               string   `json:"id"`
	Hosts                            []string `json:"hosts"`
	MinimumRegisterIntervalInSeconds int      `json:"minimumRegisterIntervalInSeconds"`
}
