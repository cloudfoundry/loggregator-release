package websocketserver_test

import (
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"github.com/apoydence/eachers/testhelpers"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
)

var _ = Describe("WebsocketServer", func() {
	var (
		logger         = loggertesthelper.Logger()
		server         *websocketserver.WebsocketServer
		sinkManager    = sinkmanager.New(1024, false, blacklist.New(nil, logger), logger, 100, "dropsonde-origin", 1*time.Second, 0, 1*time.Second, 500*time.Millisecond, false)
		appId          = "my-app"
		wsReceivedChan chan []byte
		apiEndpoint    string
		mockBatcher    *mockBatcher
		mockChainer    *mockBatchCounterChainer
	)

	BeforeEach(func() {
		mockBatcher = newMockBatcher()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		wsReceivedChan = make(chan []byte, 100)

		// TODO: This is a poor solution that we need to remove ASAP. Ideally,
		// we can rely on the kernel to give us an open port and pass the
		// listener directly to websocket.New call.
		retries := 5
		for ; retries > 0; retries-- {
			apiEndpoint = net.JoinHostPort("127.0.0.1", strconv.Itoa(getPort()+(config.GinkgoConfig.ParallelNode*10)))
			var err error
			server, err = websocketserver.New(
				apiEndpoint,
				sinkManager,
				100*time.Millisecond,
				100*time.Millisecond,
				100,
				"dropsonde-origin",
				mockBatcher,
				logger,
			)

			if err == nil {
				break
			}
		}
		Expect(retries).To(BeNumerically(">", 0))

		go server.Start()
		websocket.DefaultDialer = &websocket.Dialer{HandshakeTimeout: 10 * time.Millisecond}
	})

	AfterEach(func() {
		server.Stop()
	})

	It("fails without an appId", func() {
		_, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/apps//stream", apiEndpoint), nil)
		Expect(err).To(HaveOccurred())
	})

	It("fails with bad path", func() {
		_, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/apps/my-app/junk", apiEndpoint), nil)
		Expect(err).To(HaveOccurred())
	})

	It("dumps buffer data to the websocket client with /recentlogs", func() {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		_, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/recentlogs", apiEndpoint, appId))
		defer cleanup()

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
	})

	It("sends sentEnvelopes metrics for /recentlogs", func() {
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		_, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/recentlogs", apiEndpoint, appId))
		defer cleanup()

		Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
			With("sentEnvelopes"),
		))
		Eventually(mockChainer.SetTagInput).Should(BeCalled(
			With("protocol", "ws"),
			With("event_type", "LogMessage"),
			With("endpoint", "recentlogs"),
		))
		Eventually(mockChainer.IncrementCalled).Should(BeCalled())
	})

	It("dumps container metric data to the websocket client with /containermetrics", func() {
		cm := factories.NewContainerMetric(appId, 0, 42.42, 1234, 123412341234)
		envelope, _ := emitter.Wrap(cm, "origin")
		sinkManager.SendTo(appId, envelope)

		_, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/containermetrics", apiEndpoint, appId))
		defer cleanup()

		rcm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rcm.GetContainerMetric()).To(Equal(cm))
	})

	It("sends sentEnvelopes metrics for /containermetrics", func() {
		cm := factories.NewContainerMetric(appId, 0, 42.42, 1234, 123412341234)
		envelope, _ := emitter.Wrap(cm, "origin")
		sinkManager.SendTo(appId, envelope)

		_, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/containermetrics", apiEndpoint, appId))
		defer cleanup()

		Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
			With("sentEnvelopes"),
		))
		Eventually(mockChainer.SetTagInput).Should(BeCalled(
			With("protocol", "ws"),
			With("event_type", "ContainerMetric"),
			With("endpoint", "containermetrics"),
		))
		Eventually(mockChainer.IncrementCalled).Should(BeCalled())
	})

	It("skips sending data to the websocket client with a marshal error", func() {
		cm := factories.NewContainerMetric(appId, 0, 42.42, 1234, 123412341234)
		cm.InstanceIndex = nil
		envelope, _ := emitter.Wrap(cm, "origin")
		sinkManager.SendTo(appId, envelope)

		_, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/containermetrics", apiEndpoint, appId))
		defer cleanup()
		Consistently(wsReceivedChan).ShouldNot(Receive())
	})

	It("sends data to the websocket client with /stream", func() {
		stopKeepAlive, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		defer cleanup()
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
	})

	It("sends sentEnvelopes metrics for /stream", func() {
		stopKeepAlive, _, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		defer cleanup()
		defer close(stopKeepAlive)
		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
			With("sentEnvelopes"),
		))
		Eventually(mockChainer.SetTagInput).Should(BeCalled(
			With("protocol", "ws"),
			With("event_type", "LogMessage"),
			With("endpoint", "stream"),
		))
		Eventually(mockChainer.IncrementCalled).Should(BeCalled())
	})

	Context("websocket firehose client", func() {
		var (
			stopKeepAlive     chan struct{}
			connectionDropped <-chan struct{}
			lm                *events.Envelope
			cleanup           func()
		)

		const subscriptionID = "firehose-subscription-a"

		BeforeEach(func() {
			stopKeepAlive, connectionDropped, cleanup = addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/firehose/%s", apiEndpoint, subscriptionID))

			lm, _ = emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		})

		AfterEach(func() {
			close(stopKeepAlive)
			<-connectionDropped
			cleanup()
		})

		It("sends data to the websocket firehose client", func() {
			sinkManager.SendTo(appId, lm)

			rlm, err := receiveEnvelope(wsReceivedChan)
			Expect(err).NotTo(HaveOccurred())
			Expect(rlm.GetLogMessage().GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		})

		Context("when data is sent to the websocket firehose client", func() {
			JustBeforeEach(func() {
				sinkManager.SendTo(appId, lm)
			})

			It("emits counter metric sentMessagesFirehose", func() {
				Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
					With("sentMessagesFirehose"),
				))

				Eventually(mockChainer.SetTagInput).Should(BeCalled(
					With("subscription_id", subscriptionID),
				))
			})

			It("emits counter metric sentEnvelopes", func() {
				Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(
					With("sentEnvelopes"),
				))

				Eventually(mockChainer.SetTagInput).Should(BeCalled(
					With("protocol", "ws"), // TODO: consider adding wss?
					With("event_type", "LogMessage"),
					With("endpoint", "firehose"),
				))
				Eventually(mockChainer.IncrementCalled).Should(BeCalled())
			})
		})
	})

	It("sends each message to only one of many firehoses with the same subscription id", func() {
		firehoseAChan1 := make(chan []byte, 100)
		stopKeepAlive1, _, cleanup := addWSSink(firehoseAChan1, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))
		defer cleanup()

		firehoseAChan2 := make(chan []byte, 100)
		stopKeepAlive2, _, cleanup := addWSSink(firehoseAChan2, fmt.Sprintf("ws://%s/firehose/fire-subscription-x", apiEndpoint))
		defer cleanup()

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

	It("works with malformed firehose path", func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/firehose", apiEndpoint))
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		bytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(bytes).To(ContainSubstring("missing subscription id in firehose request"))
	})

	It("still sends to 'live' sinks", func() {
		stopKeepAlive, connectionDropped, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		defer cleanup()
		Consistently(connectionDropped, 0.2).ShouldNot(BeClosed())

		lm, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "my message", appId, "App"), "origin")
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveEnvelope(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).ToNot(BeNil())
		close(stopKeepAlive)
	})

	It("closes the client when the keep-alive stops", func() {
		stopKeepAlive, connectionDropped, cleanup := addWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId))
		defer cleanup()
		Expect(stopKeepAlive).ToNot(Receive())
		close(stopKeepAlive)
		Eventually(connectionDropped).Should(BeClosed())
	})

	It("times out slow connections", func() {
		errChan := make(chan error)
		url := fmt.Sprintf("ws://%s/apps/%s/stream", apiEndpoint, appId)
		addSlowWSSink(wsReceivedChan, errChan, 2*time.Second, url)
		var err error
		Eventually(errChan, 5).Should(Receive(&err))
		Expect(err).To(HaveOccurred())
	})
})

func receiveEnvelope(dataChan <-chan []byte) (*events.Envelope, error) {
	var data []byte
	Eventually(dataChan, 2).Should(Receive(&data))
	return parseEnvelope(data)
}

func getPort() int {
	portRangeStart := 55000
	portRangeCoefficient := 100
	offset := 5
	return config.GinkgoConfig.ParallelNode*portRangeCoefficient + portRangeStart + offset
}
