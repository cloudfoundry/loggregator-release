package legacyproxy_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"trafficcontroller/dopplerproxy"
	"trafficcontroller/legacyproxy"
)

var _ = Describe("LegacyHandlerProvider", func() {
	var (
		innerHandler          http.Handler
		factory               *fakeHandlerProviderFactory
		legacyHandlerProvider dopplerproxy.HandlerProvider
	)

	BeforeEach(func() {
		innerHandler = &dummyHandler{}
		factory = &fakeHandlerProviderFactory{returnedHandler: innerHandler}
		legacyHandlerProvider = legacyproxy.NewLegacyHandlerProvider(factory.fakeHandlerProvider, loggertesthelper.Logger())
	})

	It("delegates to the wrapped handler provider", func() {
		legacyHandler := legacyHandlerProvider("fake-endpoint", make(chan []byte))

		Expect(factory.endpoint).To(Equal("fake-endpoint"))
		Expect(legacyHandler).To(Equal(innerHandler))
	})

	It("translates messages into the legacy format", func() {
		var messageChan = make(chan []byte, 1)
		legacyHandlerProvider("fake-endpoint", messageChan)

		messageChan <- makeDropsondeMessage("message", "app-id", 123)
		legacyMessage := makeLegacyMessage("message", "app-id", 123)

		Eventually(factory.messages).Should(Receive(Equal(legacyMessage)))
	})

	It("drops messages that can't be translated", func() {
		var messageChan = make(chan []byte, 1)
		legacyHandlerProvider("fake-endpoint", messageChan)

		messageChan <- []byte{1, 2, 3}

		Consistently(factory.messages).ShouldNot(Receive())
	})
})

type fakeHandlerProviderFactory struct {
	endpoint        string
	messages        <-chan []byte
	returnedHandler http.Handler
}

func (factory *fakeHandlerProviderFactory) fakeHandlerProvider(endpoint string, messages <-chan []byte) http.Handler {
	factory.endpoint = endpoint
	factory.messages = messages
	return factory.returnedHandler
}

type dummyHandler struct {
}

func (*dummyHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func makeLegacyMessage(messageString string, appId string, currentTime int64) []byte {
	messageType := logmessage.LogMessage_ERR
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		MessageType: &messageType,
		Timestamp:   proto.Int64(currentTime),
		AppId:       proto.String(appId),
		SourceName:  proto.String("DOP"),
		SourceId:    proto.String("SN"),
	}

	msg, _ := proto.Marshal(logMessage)
	return msg
}

func makeDropsondeMessage(messageString string, appId string, currentTime int64) []byte {
	logMessage := &events.LogMessage{
		Message:        []byte(messageString),
		MessageType:    events.LogMessage_ERR.Enum(),
		Timestamp:      proto.Int64(currentTime),
		AppId:          proto.String(appId),
		SourceType:     proto.String("DOP"),
		SourceInstance: proto.String("SN"),
	}

	envelope, _ := emitter.Wrap(logMessage, "doppler")
	msg, _ := proto.Marshal(envelope)

	return msg
}
