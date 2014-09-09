package websocketserver_test

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/sinkmanager"
	"loggregator/sinkserver/websocketserver"
	"net/http"
	"time"
)

var _ = Describe("WebsocketServer", func() {

	var server *websocketserver.WebsocketServer
	var sinkManager, _ = sinkmanager.NewSinkManager(1024, false, blacklist.New(nil), loggertesthelper.Logger(), 1*time.Hour)
	var appId = "my-app"
	var wsReceivedChan = make(chan []byte)
	var connectionDropped <-chan struct{}
	var apiEndpoint = "127.0.0.1:9091"

	BeforeEach(func() {
		logger := loggertesthelper.Logger()
		cfcomponent.Logger = logger

		server = websocketserver.New(apiEndpoint, sinkManager, 100*time.Millisecond, 100, logger)
		go server.Start()
		serverUrl := fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId)
		Eventually(func() error { _, _, err := websocket.DefaultDialer.Dial(serverUrl, http.Header{}); return err }).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Stop()
		time.Sleep(1 * time.Millisecond)
	})

	Describe("failed connections", func() {
		It("should fail without an appId", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("should fail with an invalid appId", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("should fail with something invalid in query string", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?something=invalidtarget", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("should fail with bad path", func() {
			_, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/bad_path/", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})
	})

	It("should dump buffer data to the websocket client", func(done Done) {
		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/dump/?app=%s", apiEndpoint, appId))

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(done)
	})

	It("should dump buffer data to the websocket client with /recent", func(done Done) {
		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/recent?app=%s", apiEndpoint, appId))

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(done)
	})

	It("should send data to the websocket client", func(done Done) {
		stopKeepAlive, _ := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm.GetMessage()).To(Equal(lm.GetLogMessage().GetMessage()))
		close(stopKeepAlive)
		close(done)
	})

	It("should still send to 'live' sinks", func(done Done) {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		Consistently(connectionDropped, 0.2).ShouldNot(BeClosed())

		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).ToNot(BeNil())
		close(stopKeepAlive)
		close(done)
	})

	It("should close the client when the keep-alive stops", func() {
		stopKeepAlive, connectionDropped := AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		Expect(stopKeepAlive).ToNot(Receive())
		close(stopKeepAlive)
		Eventually(connectionDropped).Should(BeClosed())
	})
})

func receiveLogMessage(dataChan <-chan []byte) (*logmessage.LogMessage, error) {
	receivedData := <-dataChan
	return parseLogMessage(receivedData)
}
