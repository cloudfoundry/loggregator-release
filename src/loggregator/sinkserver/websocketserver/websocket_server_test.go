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
	"time"
)

var _ = Describe("WebsocketServer", func() {

	var server *websocketserver.WebsocketServer
	var tailClient *websocket.Conn
	var sinkManager, _ = sinkmanager.NewSinkManager(1024, false, blacklist.New(nil), loggertesthelper.Logger())
	var appId = "my-app"
	var wsReceivedChan chan []byte
	var stopKeepAlive chan<- struct{}
	var connectionDropped <-chan struct{}
	var apiEndpoint = "127.0.0.1:9091"

	BeforeEach(func(done Done) {
		var err error
		wsReceivedChan = make(chan []byte)
		logger := loggertesthelper.Logger()
		cfcomponent.Logger = logger

		server = websocketserver.New(apiEndpoint, sinkManager, 10*time.Millisecond, 100, logger)
		go server.Start()
		<-time.After(5 * time.Millisecond)
		tailClient, stopKeepAlive, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=%s", apiEndpoint, appId))
		Expect(err).NotTo(HaveOccurred())
		close(done)
	})

	AfterEach(func() {
		server.Stop()
	})

	Describe("failed connections", func() {
		It("should fail without an appId", func() {
			_, _, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("should fail with an invalid appId", func() {
			_, _, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?app=", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})

		It("should fail with something invalid", func() {
			_, _, connectionDropped = AddWSSink(wsReceivedChan, fmt.Sprintf("ws://%s/tail/?something=invalidtarget", apiEndpoint))
			Expect(connectionDropped).To(BeClosed())
		})
	})

	It("should send data to the websocket client", func(done Done) {
		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).To(Equal(lm.GetLogMessage()))
		close(done)
	})

	It("should still send to 'live' sinks", func(done Done) {
		<-time.After(200 * time.Millisecond)
		Expect(connectionDropped).ToNot(BeClosed())

		lm, err := NewMessageWithError("my message", appId)
		Expect(err).NotTo(HaveOccurred())
		sinkManager.SendTo(appId, lm)

		rlm, err := receiveLogMessage(wsReceivedChan)
		Expect(err).NotTo(HaveOccurred())
		Expect(rlm).ToNot(BeNil())
		close(done)
	})

	It("should close the client when the keep-alive stops", func(done Done) {
		go func() {
			for _ = range wsReceivedChan {
			}
		}()

		go func() {
			for {
				lm, err := NewMessageWithError("my message", appId)
				Expect(err).NotTo(HaveOccurred())
				sinkManager.SendTo(appId, lm)
				time.Sleep(2 * time.Millisecond)
			}
		}()

		time.Sleep(10 * time.Millisecond) //wait a little bit to make sure some messages are sent

		close(stopKeepAlive)

		Eventually(connectionDropped).Should(BeClosed())
		close(done)
	})
})

func receiveLogMessage(dataChan <-chan []byte) (*logmessage.LogMessage, error) {
	receivedData := <-dataChan
	return parseLogMessage(receivedData)
}
