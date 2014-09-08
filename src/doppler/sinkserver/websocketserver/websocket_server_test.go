package websocketserver_test

import (
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	"net/http"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {

	var server *websocketserver.WebsocketServer
	var sinkManager, _ = sinkmanager.NewSinkManager(1024, false, blacklist.New(nil), loggertesthelper.Logger(), "dropsonde-origin")
	var appId = "my-app"
	var wsReceivedChan chan []byte
	var connectionDropped <-chan struct{}
	var apiEndpoint = "127.0.0.1:9091"

	BeforeEach(func() {
		logger := loggertesthelper.Logger()
		cfcomponent.Logger = logger
		wsReceivedChan = make(chan []byte)

		server = websocketserver.New(apiEndpoint, sinkManager, 100*time.Millisecond, 100, logger)
		go server.Start()
		serverUrl := fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId)
		Eventually(func() error { _, _, err := websocket.DefaultDialer.Dial(serverUrl, http.Header{}); return err }, 1).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Stop()
	})

	Describe("failed connections", func() {
		It("fails without an appId", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("fails with an invalid appId", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("fails with something invalid in query string", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?something=invalidtarget", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("fails with bad path", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/bad_path/", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})
	})

	It("dumps buffer data to the websocket client", func(done Done) {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/dump/?app=%s", apiEndpoint, appId))

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(done)
	})

	It("dumps buffer data to the websocket client with /recent", func(done Done) {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/recent?app=%s", apiEndpoint, appId))

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(done)
	})

	It("sends data to the websocket client", func(done Done) {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
		close(done)
	})

	It("sends data to the websocket firehose client", func(done Done) {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/firehose", apiEndpoint))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
		close(done)
	})

	It("still sends to 'live' sinks", func(done Done) {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		Consistently(connectionDropped, 0.2).ShouldNot(BeClosed())

		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).ToNot(BeNil())
		close(stopKeepAlive)
		close(done)
	})

	It("closes the client when the keep-alive stops", func() {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		Expect(stopKeepAlive).ToNot(Receive())
		close(stopKeepAlive)
		Eventually(connectionDropped).Should(BeClosed())
	})
})

func receiveLogMessage(dataChan <-chan []byte) (*events.Envelope, error) {
	receivedData := <-dataChan
	return parseLogMessage(receivedData)
}
