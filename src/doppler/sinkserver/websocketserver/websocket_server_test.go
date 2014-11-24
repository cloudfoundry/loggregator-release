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
	var sinkManager = sinkmanager.NewSinkManager(1024, false, blacklist.New(nil), loggertesthelper.Logger(), "dropsonde-origin")
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
		serverUrl := fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId)
		websocket.DefaultDialer = &websocket.Dialer{HandshakeTimeout: 10 * time.Millisecond}
		Eventually(func() error { _, _, err := websocket.DefaultDialer.Dial(serverUrl, http.Header{}); return err }, 1).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Stop()
		time.Sleep(time.Millisecond * 10)
	})

	Describe("failed connections", func() {
		It("fails without an appId", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps//stream", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("fails with bad path", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/my-app/junk", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})
	})

	It("dumps buffer data to the websocket client with /recentlogs", func(done Done) {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/recentlogs", apiEndpoint, appId))

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(done)
	})

	It("sends data to the websocket client", func(done Done) {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
		close(done)
	})

	It("sends data to the websocket firehose client", func(done Done) {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/firehose/fire-subscription-a", apiEndpoint))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
		close(done)
	})

	It("sends each message to only one of many firehoses with the same subscription id", func(done Done) {
		firehoseAChan1 := make(chan []byte, 100)
		stopKeepAlive1, _ := AddWSSink(firehoseAChan1, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))

		firehoseAChan2 := make(chan []byte, 100)
		stopKeepAlive2, _ := AddWSSink(firehoseAChan2, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))

		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		for i := 0; i < 100; i++ {
			sinkManager.SendTo(appId, lm)
		}

		Eventually(func() int {
			return len(firehoseAChan1) + len(firehoseAChan2)
		}).Should(Equal(100))

		Consistently(func() int {
			return len(firehoseAChan2)
		}).Should(BeNumerically(">", 0))

		Consistently(func() int {
			return len(firehoseAChan1)
		}).Should(BeNumerically(">", 0))

		close(stopKeepAlive1)
		close(stopKeepAlive2)
		close(done)
	}, 2)

	It("still sends to 'live' sinks", func(done Done) {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
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
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		Expect(stopKeepAlive).ToNot(Receive())
		close(stopKeepAlive)
		Eventually(connectionDropped).Should(BeClosed())
	})
})

func receiveLogMessage(dataChan <-chan []byte) (*events.Envelope, error) {
	receivedData := <-dataChan
	return parseLogMessage(receivedData)
}
