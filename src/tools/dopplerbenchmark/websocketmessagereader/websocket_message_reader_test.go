package websocketmessagereader_test

import (
	"tools/dopplerbenchmark/websocketmessagereader"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http/httptest"
	"time"
)

var _ = Describe("Websocketmessagereader", func() {

	It("connects to a websocket endpoint", func() {
		sentMessage := "a message"

		messages := make(chan []byte, 1)
		wsh := handlers.NewWebsocketHandler(messages, time.Second, loggertesthelper.Logger())
		server := httptest.NewServer(wsh)
		defer server.Close()

		messages <- []byte(sentMessage)

		reporter := &fakeReporter{}
		reader := websocketmessagereader.New(server.Listener.Addr().String(), reporter)

		Eventually(reader.ReadAndReturn).Should(BeEquivalentTo(sentMessage))
	})

})

type fakeReporter struct {
	totalReceived int
}

func (f *fakeReporter) IncrementReceivedMessages() {
	f.totalReceived++
}
