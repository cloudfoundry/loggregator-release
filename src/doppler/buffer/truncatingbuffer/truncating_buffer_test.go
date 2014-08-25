package truncatingbuffer_test

import (
	"doppler/buffer/truncatingbuffer"
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Truncating Buffer", func() {
	It("works like a channel", func() {
		inMessageChan := make(chan *envelopewrapper.WrappedEnvelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 2, loggertesthelper.Logger())
		go buffer.Run()

		logMessage1, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "message 1", "appId", "App"), "origin")
		inMessageChan <- logMessage1
		readMessage := <-buffer.GetOutputChannel()
		Expect(readMessage.Envelope.GetLogMessage().GetMessage()).To(ContainSubstring("message 1"))

		logMessage2, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "message 2", "appId", "App"), "origin")

		inMessageChan <- logMessage2
		readMessage2 := <-buffer.GetOutputChannel()
		Expect(readMessage2.Envelope.GetLogMessage().GetMessage()).To(ContainSubstring("message 2"))

	})

	It("works like a truncating channel", func() {
		inMessageChan := make(chan *envelopewrapper.WrappedEnvelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 2, loggertesthelper.Logger())
		go buffer.Run()

		logMessage1, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "message 1", "appId", "App"), "origin")

		inMessageChan <- logMessage1

		logMessage2, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "message 2", "appId", "App"), "origin")
		inMessageChan <- logMessage2

		logMessage3, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "message 3", "appId", "App"), "origin")
		inMessageChan <- logMessage3
		time.Sleep(5 * time.Millisecond)

		readMessage := <-buffer.GetOutputChannel()
		Expect(readMessage.Envelope.GetLogMessage().GetMessage()).To(ContainSubstring("Log message output too high. We've dropped 2 messages"))

		readMessage2 := <-buffer.GetOutputChannel()
		Expect(readMessage2.Envelope.GetLogMessage().GetMessage()).To(ContainSubstring("message 3"))
	})
})
