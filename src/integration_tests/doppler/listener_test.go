package doppler_test

import (
	. "integration_tests/doppler/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"net"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/localip"
)

var _ = Describe("Listener test", func() {
	Context("with TLS Listener config specified", func() {
		var address string
		var ws *websocket.Conn
		var conn net.Conn
		var receiveChan chan []byte

		BeforeEach(func() {
			ip, _ := localip.LocalIP()
			address = fmt.Sprintf("%s:%d", ip, 8766)
			conn = openTLSConnection(address)

			receiveChan = make(chan []byte, 10)
			ws, _ = AddWSSink(receiveChan, "4567", "/firehose/hose-subcription-a")
		})

		AfterEach(func() {
			receiveChan = nil
			ws.Close()
		})

		It("listens for dropsonde log message on TLS port", func() {
			message := "my-random-tls-message"
			guid, _ := uuid.NewV4()
			appID := guid.String()

			logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")
			SendEventTLS(logMessage, conn)

			receivedMessageBytes := []byte{}
			Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

			receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)
			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(Equal(message))
		})

		It("listens for dropsonde counter event on TLS port", func() {
			counterEvent := factories.NewCounterEvent("my-counter", 1)
			SendEventTLS(counterEvent, conn)

			receivedEventBytes := []byte{}
			Eventually(receiveChan).Should(Receive(&receivedEventBytes))

			receivedEvent := DecodeProtoBufCounterEvent(receivedEventBytes)
			Expect(receivedEvent.GetName()).To(Equal("my-counter"))
			Expect(receivedEvent.GetDelta()).To(Equal(uint64(1)))
		})
	})
})

func openTLSConnection(address string) net.Conn {

	var conn net.Conn
	var err error
	Eventually(func() error {
		conn, err = net.Dial("tcp", address)
		return err
	}).ShouldNot(HaveOccurred())

	return conn
}
