package integration_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"time"
	"trafficcontroller/integration_test/fake_auth_server"
	"trafficcontroller/integration_test/fake_doppler"
	"trafficcontroller/integration_test/fake_uaa_server"
	"trafficcontroller/integration_test/traffic_controller_client"

	"github.com/cloudfoundry/loggregator_consumer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var command *exec.Cmd
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string
var fakeDoppler *fake_doppler.FakeDoppler

const DOPPLER_LEGACY_PORT = 4567
const DOPPLER_DROPSONDE_PORT = 4566

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
		trafficControllerDropsondeEndpoint := fmt.Sprintf("http://%s:%d", localIPAddress, DOPPLER_DROPSONDE_PORT)
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

var _ = Describe("TrafficController", func() {
	BeforeEach(func() {
		fakeDoppler.ResetMessageChan()
	})

	Context("Streaming", func() {
		Context("dropsonde messages", func() {
			It("passes messages through", func() {
				client := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%s:%d/%s", localIPAddress, DOPPLER_DROPSONDE_PORT, "apps/1234/stream")}
				go client.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

				messageBody := []byte("CUSTOM Doppler MESSAGE")
				fakeDoppler.SendLogMessage(messageBody)
				Eventually(func() bool {
					return client.DidReceiveLegacyMessage(messageBody)
				}).Should(BeTrue())

				client.Stop()
			})
		})

		Context("legacy messages", func() {
			It("delivers legacy format messages at legacy endpoint", func() {
				client := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%s:%d/%s", localIPAddress, DOPPLER_LEGACY_PORT, "tail/?app=1234")}
				go client.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

				currentTime := time.Now().UnixNano()
				dropsondeMessage := makeDropsondeMessage("Make me Legacy Format", "1234", currentTime)
				legacyMessage := makeLegacyMessage("Make me Legacy Format", "1234", currentTime)
				fakeDoppler.SendLogMessage(dropsondeMessage)

				Eventually(func() bool {
					return client.DidReceiveLegacyMessage(legacyMessage)
				}).Should(BeTrue())

				client.Stop()
			})

			It("returns bad handshake for invalid legacy endpoint", func() {
				client := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%s:%d/%s", localIPAddress, DOPPLER_LEGACY_PORT, "invalid-path?app=1234")}

				err, resp := client.Start()
				Expect(err).To(Equal(errors.New("websocket: bad handshake")))
				errMsg, _ := ioutil.ReadAll(resp.Body)
				Expect(errMsg).To(BeEquivalentTo("invalid request: unexpected path: /invalid-path\n"))
			})
		})

		Context("Firehose", func() {
			It("passes messages through for every app for uaa admins", func() {
				client := &traffic_controller_client.TrafficControllerClient{ApiEndpoint: fmt.Sprintf("ws://%s:%d/%s", localIPAddress, DOPPLER_DROPSONDE_PORT, "firehose")}
				go client.Start()
				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/firehose"))

				messageBody := []byte("CUSTOM Doppler MESSAGE")
				fakeDoppler.SendLogMessage(messageBody)
				Eventually(func() bool {
					return client.DidReceiveLegacyMessage(messageBody)
				}, 5).Should(BeTrue())

				client.Stop()
			})
		})
	})

	Context("Recent", func() {
		var expectedMessages [][]byte
		BeforeEach(func() {
			expectedMessages = make([][]byte, 5)

			for i := 0; i < 5; i++ {
				message := makeDropsondeMessage(strconv.Itoa(i), "1234", 1234)
				expectedMessages[i] = message
				fakeDoppler.SendLogMessage(message)
			}
			fakeDoppler.CloseLogMessageStream()
		})

		Context("dropsonde messages", func() {
			It("returns a multi-part HTTP response with all recent messages", func(done Done) {
				endpoint := fmt.Sprintf("ws://%s:%d", localIPAddress, DOPPLER_DROPSONDE_PORT)
				client := noaa.NewNoaa(endpoint, &tls.Config{}, nil)

				var messages []*events.Envelope
				var err error
				go func() {
					messages, err = client.RecentLogs("1234", "bearer iAmAnAdmin")
				}()

				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 15).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/recentlogs"))

				Expect(err).NotTo(HaveOccurred())

				for i, message := range messages {
					Expect(message.GetLogMessage().GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
				}
				close(done)
			}, 20)
		})

		Context("legacy messages", func() {
			It("returns a multi-part HTTP response with all recent messages", func(done Done) {
				endpoint := fmt.Sprintf("ws://%s:%d", localIPAddress, DOPPLER_LEGACY_PORT)
				client := loggregator_consumer.New(endpoint, &tls.Config{}, nil)

				var messages []*logmessage.LogMessage
				var err error
				go func() {
					messages, err = client.Recent("1234", "bearer iAmAnAdmin")
				}()

				var request *http.Request
				Eventually(fakeDoppler.TrafficControllerConnected, 15).Should(Receive(&request))
				Expect(request.URL.Path).To(Equal("/apps/1234/recentlogs"))

				Expect(err).NotTo(HaveOccurred())

				fmt.Printf("********** got %d messages\n", len(messages))
				for i, message := range messages {
					fmt.Printf("********* message: %v\n", message)
					Expect(message.GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
				}
				close(done)
			}, 20)
		})
	})
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
