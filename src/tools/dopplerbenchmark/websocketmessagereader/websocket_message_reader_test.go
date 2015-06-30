package websocketmessagereader_test

import (
	"tools/dopplerbenchmark/websocketmessagereader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"time"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"net/http/httptest"
)

var _ = Describe("Websocketmessagereader", func() {

	It("connects to a websocket endpoint", func() {
		sentMessage := "a message"

		messages := make(chan []byte, 1)
		wsh := handlers.NewWebsocketHandler(messages, time.Second, loggertesthelper.Logger())
		server := httptest.NewServer(wsh)
		defer server.Close()

		messages <- []byte(sentMessage)

		reader := websocketmessagereader.New(server.Listener.Addr().String())

		Eventually(reader.ReadAndReturn).Should(BeEquivalentTo(sentMessage))
	})

})
