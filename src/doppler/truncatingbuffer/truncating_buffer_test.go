package truncatingbuffer_test

import (
	"doppler/truncatingbuffer"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Truncating Buffer", func() {

	It("panics if buffer size is less than 3", func() {
		inMessageChan := make(chan *events.Envelope)
		Expect(func() {
			truncatingbuffer.NewTruncatingBuffer(inMessageChan, 2, loggertesthelper.Logger(), "dropsonde-origin", "test-sync-name")
		}).To(Panic())
	})

	It("works like a channel", func() {
		inMessageChan := make(chan *events.Envelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, loggertesthelper.Logger(), "dropsonde-origin", "test-sink-name")
		go buffer.Run()

		sendLogMessages("message 1", inMessageChan)

		readMessage := <-buffer.GetOutputChannel()
		Expect(readMessage.GetLogMessage().GetMessage()).To(ContainSubstring("message 1"))

		sendLogMessages("message 2", inMessageChan)

		readMessage2 := <-buffer.GetOutputChannel()
		Expect(readMessage2.GetLogMessage().GetMessage()).To(ContainSubstring("message 2"))

	})

	It("works like a truncating channel", func() {
		inMessageChan := make(chan *events.Envelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, loggertesthelper.Logger(), "dropsonde-origin", "test-sink-name")
		go buffer.Run()

		sendLogMessages("message 1", inMessageChan)
		sendLogMessages("message 2", inMessageChan)
		sendLogMessages("message 3", inMessageChan)
		sendLogMessages("message 4", inMessageChan)

		time.Sleep(5 * time.Millisecond)

		logMessageNotification := <-buffer.GetOutputChannel()
		Expect(logMessageNotification.GetLogMessage().GetMessage()).To(ContainSubstring("Log message output too high. We've dropped 3 messages to test-sink-name."))

		counterEventNotification := <-buffer.GetOutputChannel()
		Expect(counterEventNotification.GetEventType()).To(Equal(events.Envelope_CounterEvent))
		counterEvent := counterEventNotification.GetCounterEvent()
		Expect(counterEvent.GetName()).To(Equal("TruncatingBuffer.DroppedMessages"))
		Expect(counterEvent.GetDelta()).To(BeEquivalentTo(3))
		Expect(counterEvent.GetTotal()).To(BeEquivalentTo(3))

		originalMessage4 := <-buffer.GetOutputChannel()
		Expect(originalMessage4.GetLogMessage().GetMessage()).To(ContainSubstring("message 4"))

		sendLogMessages("message 5", inMessageChan)
		sendLogMessages("message 6", inMessageChan)
		sendLogMessages("message 7", inMessageChan)
		sendLogMessages("message 8", inMessageChan)

		logMessageNotification = <-buffer.GetOutputChannel()
		Expect(logMessageNotification.GetEventType()).To(Equal(events.Envelope_LogMessage))

		counterEventNotification = <-buffer.GetOutputChannel()
		Expect(counterEventNotification.GetEventType()).To(Equal(events.Envelope_CounterEvent))
		counterEvent = counterEventNotification.GetCounterEvent()
		Expect(counterEvent.GetName()).To(Equal("TruncatingBuffer.DroppedMessages"))
		Expect(counterEvent.GetDelta()).To(BeEquivalentTo(3))
		Expect(counterEvent.GetTotal()).To(BeEquivalentTo(6))
	})

	It("keeps track of dropped messages", func(done Done) {
		inMessageChan := make(chan *events.Envelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, loggertesthelper.Logger(), "dropsonde-origin", "test-sync-name")
		Expect(buffer.GetDroppedMessageCount()).To(Equal(int64(0)))
		go buffer.Run()

		sendLogMessages("message 1", inMessageChan)
		sendLogMessages("message 2", inMessageChan)
		sendLogMessages("message 3", inMessageChan)
		sendLogMessages("message 4", inMessageChan)

		Eventually(buffer.GetDroppedMessageCount).Should(Equal(int64(3)))

		close(done)
	})

	It("updates totalDroppedMessages", func() {
		fakeEventEmitter := fake.NewFakeEventEmitter("doppler")
		sender := metric_sender.NewMetricSender(fakeEventEmitter)
		batcher := metricbatcher.New(sender, 100*time.Millisecond)

		metrics.Initialize(sender, batcher)
		fakeEventEmitter.Reset()

		inMessageChan := make(chan *events.Envelope)
		buffer := truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, loggertesthelper.Logger(), "dropsonde-origin", "test-sync-name")
		Expect(buffer.GetDroppedMessageCount()).To(Equal(int64(0)))
		go buffer.Run()

		sendLogMessages("message 1", inMessageChan)
		sendLogMessages("message 2", inMessageChan)
		sendLogMessages("message 3", inMessageChan)
		sendLogMessages("message 4", inMessageChan)

		Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
			Name:  proto.String("TruncatingBuffer.totalDroppedMessages"),
			Delta: proto.Uint64(3),
		}))
	})

})

func sendLogMessages(message string, inMessageChan chan<- *events.Envelope) {
	logMessage1, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, message, "appId", "App"), "origin")
	inMessageChan <- logMessage1
}
