package sinkserver_test

import (
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"net/http"
	testhelpers "server_testhelpers"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dumping", func() {
	var (
		sinkManager         *sinkmanager.SinkManager
		TestMessageRouter   *sinkserver.MessageRouter
		TestWebsocketServer *websocketserver.WebsocketServer
		dataReadChannel     chan *events.Envelope
		services            sync.WaitGroup
	)

	const (
		SERVER_PORT = "9081"
	)

	BeforeEach(func() {
		dataReadChannel = make(chan *events.Envelope, 2)

		logger := loggertesthelper.Logger()
		cfcomponent.Logger = logger

		newAppServiceChan := make(chan appservice.AppService)
		deletedAppServiceChan := make(chan appservice.AppService)

		emptyBlacklist := blacklist.New(nil)
		sinkManager = sinkmanager.NewSinkManager(1024, false, emptyBlacklist, logger, "dropsonde-origin",
			1*time.Second, 1*time.Second)

		services.Add(1)
		go func() {
			defer services.Done()
			sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
		}()

		TestMessageRouter = sinkserver.NewMessageRouter(sinkManager, logger)

		services.Add(1)
		go func() {
			defer services.Done()
			TestMessageRouter.Start(dataReadChannel)
		}()

		apiEndpoint := "localhost:" + SERVER_PORT
		TestWebsocketServer = websocketserver.New(apiEndpoint, sinkManager, 10*time.Second, 100, loggertesthelper.Logger())

		services.Add(1)
		go func() {
			defer services.Done()
			TestWebsocketServer.Start()
		}()

		time.Sleep(5 * time.Millisecond)
	})

	AfterEach(func() {
		sinkManager.Stop()
		TestMessageRouter.Stop()
		TestWebsocketServer.Stop()

		services.Wait()
	})

	It("dumps all messages for an app user", func() {
		expectedFirstMessageString := "Some data 1"
		lm := factories.NewLogMessage(events.LogMessage_OUT, expectedFirstMessageString, "myOtherApp", "APP")
		env1, _ := emitter.Wrap(lm, "ORIGIN")

		expectedSecondMessageString := "Some data 2"
		lm = factories.NewLogMessage(events.LogMessage_OUT, expectedSecondMessageString, "myOtherApp", "APP")
		env2, _ := emitter.Wrap(lm, "ORIGIN")

		dataReadChannel <- env1
		dataReadChannel <- env2

		receivedChan := make(chan []byte, 2)
		_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(GinkgoT(), receivedChan, SERVER_PORT, "/apps/myOtherApp/recentlogs")

		Eventually(droppedChannel).Should(Receive())

		logMessages := dumpAllMessages(receivedChan)

		Expect(logMessages).To(HaveLen(2))

		firstMarshalledEnvelope := logMessages[0]
		secondMarshalledEnvelope := logMessages[1]

		var envelope1 events.Envelope
		var envelope2 events.Envelope

		proto.Unmarshal(firstMarshalledEnvelope, &envelope1)
		proto.Unmarshal(secondMarshalledEnvelope, &envelope2)

		Expect(envelope1.GetLogMessage().GetMessage()).To(BeEquivalentTo(expectedFirstMessageString))
		Expect(envelope2.GetLogMessage().GetMessage()).To(BeEquivalentTo(expectedSecondMessageString))

		stopKeepAlive <- true
	})

	It("doesn't hang when there are no messages", func() {
		receivedChan := make(chan []byte, 1)
		testhelpers.AddWSSink(GinkgoT(), receivedChan, SERVER_PORT, "/apps/myOtherApp/recentlogs")

		doneChan := make(chan bool)
		go func() {
			dumpAllMessages(receivedChan)
			close(doneChan)
		}()

		Eventually(doneChan).Should(BeClosed())
	})

	It("errors when log target is invalid", func() {
		path := "/dump/?something=invalidtarget"
		_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+SERVER_PORT+path, http.Header{})
		Expect(err).To(HaveOccurred())
	})
})

func dumpAllMessages(receivedChan chan []byte) [][]byte {
	logMessages := [][]byte{}
	for message := range receivedChan {
		logMessages = append(logMessages, message)
	}
	return logMessages
}
