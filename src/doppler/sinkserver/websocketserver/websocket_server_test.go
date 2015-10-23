package websocketserver_test

import (
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"

	"github.com/cloudfoundry/dropsonde/emitter"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {

	var server *websocketserver.WebsocketServer
	var sinkManager = sinkmanager.New(1024, false, blacklist.New(nil), loggertesthelper.Logger(), 100, "dropsonde-origin", 1*time.Second, 0, 1*time.Second, 500*time.Millisecond)
	var appId = "my-app"
	var wsReceivedChan chan []byte
	var connectionDropped <-chan struct{}
	var apiEndpoint string

	BeforeEach(func() {
		logger := loggertesthelper.Logger()
		wsReceivedChan = make(chan []byte)

		apiEndpoint = net.JoinHostPort("127.0.0.1", strconv.Itoa(9091+config.GinkgoConfig.ParallelNode*10))
		var err error
		server, err = websocketserver.New(apiEndpoint, sinkManager, 100*time.Millisecond, 100, "dropsonde-origin", logger)
		Expect(err).NotTo(HaveOccurred())
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

	It("dumps buffer data to the websocket client with /recentlogs", func() {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/recentlogs", apiEndpoint, appId))

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
	})

	It("dumps container metric data to the websocket client with /containermetrics", func() {
		cm := factories.NewContainerMetric(appId, 0, 42.42, 1234, 123412341234)
		envelope, _ := emitter.Wrap(cm, "origin")
		sinkManager.SendTo(appId, envelope)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/containermetrics", apiEndpoint, appId))

		rcm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rcm.GetContainerMetric()).To(Equal(cm))
	})

	It("skips sending data to the websocket client with a marshal error", func() {
		cm := factories.NewContainerMetric(appId, 0, 42.42, 1234, 123412341234)
		cm.InstanceIndex = nil
		envelope, _ := emitter.Wrap(cm, "origin")
		sinkManager.SendTo(appId, envelope)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/containermetrics", apiEndpoint, appId))
		Consistently(wsReceivedChan).ShouldNot(Receive())
	})

	It("sends data to the websocket client with /stream", func() {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
	})

	It("sends data to the websocket firehose client", func() {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/firehose/fire-subscription-a", apiEndpoint))
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
	})

	It("sends each message to only one of many firehoses with the same subscription id", func() {
		firehoseAChan1 := make(chan []byte, 100)
		stopKeepAlive1, _ := AddWSSink(firehoseAChan1, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))

		firehoseAChan2 := make(chan []byte, 100)
		stopKeepAlive2, _ := AddWSSink(firehoseAChan2, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))

		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")

		for i := 0; i < 2; i++ {
			sinkManager.SendTo(appId, lm)
		}

		Eventually(func() int {
			return len(firehoseAChan1) + len(firehoseAChan2)
		}).Should(Equal(2))

		Consistently(func() int {
			return len(firehoseAChan2)
		}).Should(BeNumerically(">", 0))

		Consistently(func() int {
			return len(firehoseAChan1)
		}).Should(BeNumerically(">", 0))

		close(stopKeepAlive1)
		close(stopKeepAlive2)
	}, 2)

	It("still sends to 'live' sinks", func() {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		Consistently(connectionDropped, 0.2).ShouldNot(BeClosed())

		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).ToNot(BeNil())
		close(stopKeepAlive)
	})

	It("closes the client when the keep-alive stops", func() {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		Expect(stopKeepAlive).ToNot(Receive())
		close(stopKeepAlive)
		Eventually(connectionDropped).Should(BeClosed())
	})
})

func receiveEnvelope(dataChan <-chan []byte) (*events.Envelope, error) {
	var data []byte
	Eventually(dataChan).Should(Receive(&data))
	return parseEnvelope(data)
}
