package websocketmessagereader_test

import (
	"tools/dopplerbenchmark/websocketmessagereader"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http/httptest"
	"time"
	"tools/benchmark/metricsreporter"
)

var _ = Describe("Websocketmessagereader", func() {

	It("connects to a websocket endpoint", func() {
		sentMessage := "a message"

		messages := make(chan []byte, 1)
		wsh := handlers.NewWebsocketHandler(messages, time.Second, loggertesthelper.Logger())
		server := httptest.NewServer(wsh)
		defer server.Close()

		messages <- []byte(sentMessage)

		receivedCounter := metricsreporter.NewCounter("counter")
		reader := websocketmessagereader.New(server.Listener.Addr().String(), receivedCounter)

		Eventually(reader.ReadAndReturn).Should(BeEquivalentTo(sentMessage))
	})

})
