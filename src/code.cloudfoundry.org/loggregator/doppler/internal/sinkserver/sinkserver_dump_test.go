package sinkserver_test

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/apoydence/eachers/testhelpers"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/onsi/ginkgo/config"

	"code.cloudfoundry.org/loggregator/diodes"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/blacklist"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/sinkmanager"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/websocketserver"
	"code.cloudfoundry.org/loggregator/doppler/internal/store"
)

var _ = Describe("Dumping", func() {
	var (
		sinkManager         *sinkmanager.SinkManager
		TestMessageRouter   *sinkserver.MessageRouter
		TestWebsocketServer *websocketserver.WebsocketServer
		dataRead            *diodes.ManyToOneEnvelope
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
		dataRead = diodes.NewManyToOneEnvelope(5, nil)

		newAppServiceChan := make(chan store.AppService)
		deletedAppServiceChan := make(chan store.AppService)

		emptyBlacklist := blacklist.New(nil)
		health := newSpyHealthRegistrar()
		sinkManager = sinkmanager.New(1024, false, emptyBlacklist, 100, "dropsonde-origin",
			2*time.Second, 0, 1*time.Second, 500*time.Millisecond, nil, testhelper.NewMetricClient(), health)

		services.Add(1)
		go func(sinkManager *sinkmanager.SinkManager) {
			defer services.Done()
			sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
		}(sinkManager)

		TestMessageRouter = sinkserver.NewMessageRouter(sinkManager)
		tempMessageRouter := TestMessageRouter

		go func(dataRead *diodes.ManyToOneEnvelope) {
			tempMessageRouter.Start(dataRead)
		}(dataRead)

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
		)
		Expect(err).NotTo(HaveOccurred())

		services.Add(1)
		go func(tempWebsocketServer *websocketserver.WebsocketServer) {
			defer services.Done()
			tempWebsocketServer.Start()
		}(TestWebsocketServer)
	})

	AfterEach(func() {
		sinkManager.Stop()
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

		dataRead.Set(env1)
		dataRead.Set(env2)

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

type SpyHealthRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
}

func newSpyHealthRegistrar() *SpyHealthRegistrar {
	return &SpyHealthRegistrar{
		values: make(map[string]float64),
	}
}

func (s *SpyHealthRegistrar) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]++
}

func (s *SpyHealthRegistrar) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]--
}

func (s *SpyHealthRegistrar) Get(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}
