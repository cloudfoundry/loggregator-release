package integration_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/gomega/gexec"
	"net/http"
	"os/exec"
	"time"
	"trafficcontroller/integration_test/fake_auth_server"
	"trafficcontroller/integration_test/fake_doppler"
	"trafficcontroller/integration_test/fake_uaa_server"
	"trafficcontroller/integration_test/traffic_controller_client"

	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
)

var command *exec.Cmd
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string
var fakeDoppler *fake_doppler.FakeDoppler

var _ = BeforeSuite(func() {
	killEtcdCmd := exec.Command("pkill", "etcd")
	killEtcdCmd.Run()

	setupEtcdAdapter()
	setupDopplerInEtcd()
	setupFakeDoppler()
	setupFakeAuthServer()
	setupFakeUaaServer()

	pathToTrafficControllerExec, err := gexec.Build("trafficcontroller")
	Expect(err).ShouldNot(HaveOccurred())

	command = exec.Command(pathToTrafficControllerExec, "--config=fixtures/trafficcontroller.json", "--debug=true")
	command.Start()

	localIPAddress, _ = localip.LocalIP()

	// wait for servers to be up
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":1235")
		return err
	}).ShouldNot(HaveOccurred())
	<-fakeDoppler.TrafficControllerConnected

	Eventually(func() error {
		trafficControllerDropsondeEndpoint := "http://" + localIPAddress + ":4566"
		_, err := http.Get(trafficControllerDropsondeEndpoint)
		return err
	}).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	command.Process.Kill()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()

	fakeDoppler.Stop()
})

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

var _ = Describe("TrafficController", func() {

	Context("Streaming", func() {
		BeforeEach(func() {
			fakeDoppler.ResetMessageChan()
		})

		Context("dropsonde messages", func() {
			It("passes messages through", func() {
				trafficControllerClient1 := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%v:%v/%v", localIPAddress, 4566, "apps/1234/stream")}
				go trafficControllerClient1.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

				messageBody := []byte("CUSTOM Doppler MESSAGE")
				fakeDoppler.SendLogMessage(messageBody)
				Eventually(func() bool {
					return trafficControllerClient1.DidReceiveLegacyMessage(messageBody)
				}).Should(BeTrue())

				trafficControllerClient1.Stop()
			})
		})

		Context("legacy messages", func() {
			It("delivers legacy format messages at legacy endpoint", func() {
				trafficControllerClient2 := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%v:%v/%v", localIPAddress, 4567, "tail/?app=1234")}
				go trafficControllerClient2.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

				currentTime := time.Now().UnixNano()
				dropsondeMessage := makeDropsondeMessage("Make me Legacy Format", "1234", currentTime)
				legacyMessage := makeLegacyMessage("Make me Legacy Format", "1234", currentTime)
				fakeDoppler.SendLogMessage(dropsondeMessage)

				Eventually(func() bool {
					return trafficControllerClient2.DidReceiveLegacyMessage(legacyMessage)
				}).Should(BeTrue())

				trafficControllerClient2.Stop()
			})

			It("returns bad handshake for invalid legacy endpoint", func() {
				client3 := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%v:%v/%v", localIPAddress, 4567, "invalid-path?app=1234")}

				err, resp := client3.Start()
				Expect(err).To(Equal(errors.New("websocket: bad handshake")))
				errMsg, _ := ioutil.ReadAll(resp.Body)
				Expect(errMsg).To(BeEquivalentTo("invalid request: unexpected path: /invalid-path\n"))
			})
		})

		Context("Firehose", func() {
			It("passes messages through for every app for uaa admins", func() {
				client5 := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%v:%v/%v", localIPAddress, 4566, "firehose")}
				go client5.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/firehose"))

				messageBody := []byte("CUSTOM Doppler MESSAGE")
				fakeDoppler.SendLogMessage(messageBody)
				Eventually(func() bool {
					return client5.DidReceiveLegacyMessage(messageBody)
				}, 5).Should(BeTrue())

				client5.Stop()
			})
		})
	})
})

func makeLegacyMessage(messageString string, appId string, currentTime int64) []byte {
	messageType := logmessage.LogMessage_ERR
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		MessageType: &messageType,
		Timestamp:   proto.Int64(currentTime),
		AppId:       proto.String(appId),
		SourceName:  proto.String("DOP"),
		SourceId:    proto.String("SN"),
	}

	msg, _ := proto.Marshal(logMessage)
	return msg
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

	envelope, _ := emitter.Wrap(logMessage, "doppler")
	msg, _ := proto.Marshal(envelope)

	return msg

}
