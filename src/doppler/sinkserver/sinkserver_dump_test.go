package sinkserver_test

import (
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"net/http"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/apoydence/eachers/testhelpers"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/onsi/ginkgo/config"
)

var _ = Describe("Dumping", func() {
	var (
		sinkManager         *sinkmanager.SinkManager
		TestMessageRouter   *sinkserver.MessageRouter
		TestWebsocketServer *websocketserver.WebsocketServer
		dataReadChannel     chan *events.Envelope
		services            sync.WaitGroup
		serverPort          string
		mockBatcher         *mockBatcher
		mockChainer         *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockBatcher = newMockBatcher()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		port := 9081 + config.GinkgoConfig.ParallelNode
		serverPort = strconv.Itoa(port)
		dataReadChannel = make(chan *events.Envelope, 2)

		logger := loggertesthelper.Logger()

		newAppServiceChan := make(chan appservice.AppService)
		deletedAppServiceChan := make(chan appservice.AppService)

		emptyBlacklist := blacklist.New(nil, logger)
		sinkManager = sinkmanager.New(1024, false, emptyBlacklist, logger, 100, "dropsonde-origin",
			2*time.Second, 0, 1*time.Second, 500*time.Millisecond)

		tempSink := sinkManager
		services.Add(1)
		go func() {
			defer services.Done()
			tempSink.Start(newAppServiceChan, deletedAppServiceChan)
		}()

		TestMessageRouter = sinkserver.NewMessageRouter(logger, sinkManager)
		tempMessageRouter := TestMessageRouter

		services.Add(1)
		go func() {
			defer services.Done()
			tempMessageRouter.Start(dataReadChannel)
		}()

		apiEndpoint := "localhost:" + serverPort
		var err error
		TestWebsocketServer, err = websocketserver.New(
			apiEndpoint,
			sinkManager,
			time.Second,
			10*time.Second,
			100,
			"dropsonde-origin",
			mockBatcher,
			logger,
		)
		Expect(err).NotTo(HaveOccurred())
		tempWebsocketServer := TestWebsocketServer

		services.Add(1)
		go func() {
			defer services.Done()
			tempWebsocketServer.Start()
		}()
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

		var receivedChan chan []byte
		Eventually(func() int {
			receivedChan = make(chan []byte, 2)
			_, stopKeepAlive, droppedChannel := AddWSSink(receivedChan, serverPort, "/apps/myOtherApp/recentlogs")
			stopKeepAlive <- true
			Eventually(droppedChannel).Should(Receive())
			return len(receivedChan)
		}).Should(Equal(2))

		var firstMarshalledEnvelope, secondMarshalledEnvelope []byte
		Eventually(receivedChan).Should(Receive(&firstMarshalledEnvelope))
		Eventually(receivedChan).Should(Receive(&secondMarshalledEnvelope))

		var envelope1 events.Envelope
		var envelope2 events.Envelope

		proto.Unmarshal(firstMarshalledEnvelope, &envelope1)
		proto.Unmarshal(secondMarshalledEnvelope, &envelope2)

		Expect(envelope1.GetLogMessage().GetMessage()).To(BeEquivalentTo(expectedFirstMessageString))
		Expect(envelope2.GetLogMessage().GetMessage()).To(BeEquivalentTo(expectedSecondMessageString))
	})

	It("doesn't hang when there are no messages", func() {
		receivedChan := make(chan []byte, 1)
		AddWSSink(receivedChan, serverPort, "/apps/myOtherApp/recentlogs")

		doneChan := make(chan bool)
		go func() {
			dumpAllMessages(receivedChan)
			close(doneChan)
		}()

		Eventually(doneChan).Should(BeClosed())
	})

	It("errors when log target is invalid", func() {
		path := "/dump/?something=invalidtarget"
		_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+serverPort+path, http.Header{})
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
