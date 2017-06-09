package truncatingbuffer_test

import (
	"fmt"

	"code.cloudfoundry.org/loggregator/doppler/internal/truncatingbuffer"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type FakeContext struct{}

func (FakeContext) EventAllowed(events.Envelope_EventType) bool {
	return true
}

func (FakeContext) Destination() string {
	return "test-sink-name"
}

func (FakeContext) Origin() string {
	return "doppler"
}

func (FakeContext) AppID(*events.Envelope) string {
	return "fake-app-id"
}

type FilteredContext struct {
	filterChan chan events.Envelope_EventType
	FakeContext
}

func NewFilteredContext(filterChan chan events.Envelope_EventType) *FilteredContext {
	return &FilteredContext{filterChan: filterChan}
}

func (f *FilteredContext) EventAllowed(eventType events.Envelope_EventType) bool {
	f.filterChan <- eventType
	return true
}

type LogFilteredContext struct {
	logMsgCount  int
	truncateChan chan struct{}
	FakeContext
}

func NewLogFilteredContext(truncateChan chan struct{}) *LogFilteredContext {
	return &LogFilteredContext{logMsgCount: 0, truncateChan: truncateChan}
}

func (f *LogFilteredContext) EventAllowed(eventType events.Envelope_EventType) bool {
	if eventType == events.Envelope_LogMessage {
		f.logMsgCount++
		if f.logMsgCount == 4+1 {
			f.truncateChan <- struct{}{}
		}
	} else if eventType == events.Envelope_CounterEvent {
		f.truncateChan <- struct{}{}
	}

	return true
}

var _ = Describe("Truncating Buffer", func() {
	var inMessageChan chan *events.Envelope
	var stopChannel chan struct{}
	var bufferSize uint
	var buffer *truncatingbuffer.TruncatingBuffer
	var context truncatingbuffer.BufferContext

	BeforeEach(func() {
		metrics.Initialize(nil, nil)
		inMessageChan = make(chan *events.Envelope)
		stopChannel = make(chan struct{})
		context = &FakeContext{}
		bufferSize = 3
	})

	JustBeforeEach(func() {
		buffer = truncatingbuffer.NewTruncatingBuffer(inMessageChan, bufferSize, context, stopChannel)
	})

	AfterEach(func() {
		if inMessageChan != nil {
			close(inMessageChan)
		}
	})

	It("panics if buffer size is less than 3", func() {
		Expect(func() {
			truncatingbuffer.NewTruncatingBuffer(inMessageChan, 2, context, nil)
		}).To(Panic())
	})

	It("panics if context is nil", func() {
		Expect(func() {
			truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, nil, nil)
		}).To(Panic())
	})

	Describe("Run", func() {
		It("exits when the input channel is closed", func() {
			done := make(chan struct{})
			go func() {
				buffer.Run()
				close(done)
			}()

			close(inMessageChan)
			Eventually(done).Should(BeClosed())
			inMessageChan = nil
		})

		It("exits when stopped", func() {
			done := make(chan struct{})
			go func() {
				buffer.Run()
				close(done)
			}()

			close(stopChannel)
			Eventually(done).Should(BeClosed())
		})
	})

	Context("as a channel", func() {
		JustBeforeEach(func() {
			go buffer.Run()
		})

		It("works like a channel", func() {

			sendLogMessages("message 1", inMessageChan)

			var readMessage *events.Envelope
			Eventually(buffer.GetOutputChannel).Should(Receive(&readMessage))

			Expect(readMessage.GetLogMessage().GetMessage()).To(ContainSubstring("message 1"))

			sendLogMessages("message 2", inMessageChan)

			Eventually(buffer.GetOutputChannel).Should(Receive(&readMessage))
			Expect(readMessage.GetLogMessage().GetMessage()).To(ContainSubstring("message 2"))

		})

		Context("tracking dropped messages", func() {
			var (
				fakeEventEmitter *fake.FakeEventEmitter
				mockBatcher      *mockMetricBatcher
			)

			tracksDroppedMessagesAnd := func(itMsg string, delta, total int) {
				It(itMsg, func() {
					var logMessageNotification *events.Envelope
					Eventually(buffer.GetOutputChannel).Should(Receive(&logMessageNotification))
					Expect(logMessageNotification.GetEventType()).To(Equal(events.Envelope_LogMessage))
					Expect(logMessageNotification.GetLogMessage().GetAppId()).To(Equal("fake-app-id"))
					Expect(logMessageNotification.GetLogMessage().GetMessage()).To(
						ContainSubstring(fmt.Sprintf("Log message output is too high. "+
							"%d messages dropped (Total %d messages dropped) from doppler to test-sink-name.", delta, total)),
					)

					var counterEventNotification *events.Envelope
					Eventually(buffer.GetOutputChannel).Should(Receive(&counterEventNotification))
					Expect(counterEventNotification.GetEventType()).To(Equal(events.Envelope_CounterEvent))
					counterEvent := counterEventNotification.GetCounterEvent()
					Expect(counterEvent.GetName()).To(Equal("TruncatingBuffer.DroppedMessages"))
					Expect(counterEvent.GetDelta()).To(BeEquivalentTo(delta))
					Expect(counterEvent.GetTotal()).To(BeEquivalentTo(total))
				})

			}

			BeforeEach(func() {
				fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
				sender := metric_sender.NewMetricSender(fakeEventEmitter)
				mockBatcher = newMockMetricBatcher()

				metrics.Initialize(sender, mockBatcher)

				fakeEventEmitter.Reset()
			})

			JustBeforeEach(func() {
				sendLogMessages("message 1", inMessageChan)
				sendLogMessages("message 2", inMessageChan)
				sendLogMessages("message 3", inMessageChan)
				sendLogMessages("message 4", inMessageChan)

				Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
					With("TruncatingBuffer.totalDroppedMessages", BeNumerically(">=", 3)),
				))
			})

			Context("when the buffer fills once", func() {
				tracksDroppedMessagesAnd("drops all the messages", 3, 3)
			})

			Context("when the buffer fills multiple times ", func() {
				var receiveEvents int
				var sendLog int

				JustBeforeEach(func() {
					outputChannel := buffer.GetOutputChannel()

					for i := 0; i < receiveEvents; i++ {
						Eventually(outputChannel).Should(Receive())
					}

					for i := 0; i < sendLog; i++ {
						sendLogMessages("message X", inMessageChan)
					}
				})

				Context("no event is read and buffer fills a second time", func() {
					BeforeEach(func() {
						receiveEvents = 0
						sendLog = 1
					})

					JustBeforeEach(func() {
						Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
							With("TruncatingBuffer.totalDroppedMessages"),
						))
					})

					tracksDroppedMessagesAnd("drops immediately", 1, 4)
				})

				Context("no event is read and buffer fills a third time", func() {
					BeforeEach(func() {
						receiveEvents = 0
						sendLog = 2
					})

					JustBeforeEach(func() {
						Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
							With("TruncatingBuffer.totalDroppedMessages"),
							With("TruncatingBuffer.totalDroppedMessages"),
						))
					})

					tracksDroppedMessagesAnd("drops immediately", 1, 5)
				})

				Context("and the TB log event is read", func() {
					BeforeEach(func() {
						receiveEvents = 1
						sendLog = 2
					})

					JustBeforeEach(func() {
						Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
							With("TruncatingBuffer.totalDroppedMessages"),
						))
					})

					tracksDroppedMessagesAnd("has 1 slot filled", 2, 5)
				})

				Context("and the TB log and counter event is read", func() {
					BeforeEach(func() {
						receiveEvents = 2
						sendLog = 3
					})

					JustBeforeEach(func() {
						Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
							With("TruncatingBuffer.totalDroppedMessages"),
						))
					})

					tracksDroppedMessagesAnd("has an empty buffer", 3, 6)
				})

			})

		})

		Context("when a filter is provided", func() {
			var filterChan chan events.Envelope_EventType

			BeforeEach(func() {
				filterChan = make(chan events.Envelope_EventType)
				context = NewFilteredContext(filterChan)
			})

			It("filters are invoked per event", func() {

				sendLogMessages("message 1", inMessageChan)

				var filteredType events.Envelope_EventType
				Eventually(filterChan).Should(Receive(&filteredType))
				Expect(filteredType).To(Equal(events.Envelope_LogMessage))
			})

			Context("and the buffer overflows", func() {
				var truncateChan chan struct{}

				BeforeEach(func() {
					truncateChan = make(chan struct{})
					context = NewLogFilteredContext(truncateChan)
				})

				It("filters truncation events", func() {
					sendLogMessages("message 1", inMessageChan)
					sendLogMessages("message 2", inMessageChan)
					sendLogMessages("message 3", inMessageChan)
					sendLogMessages("message 4", inMessageChan)

					Eventually(truncateChan).Should(Receive())
					Eventually(truncateChan).Should(Receive())
				})
			})
		})
	})
})

func sendLogMessages(message string, inMessageChan chan<- *events.Envelope) {
	logMessage1, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, message, "appId", "App"), "origin")
	inMessageChan <- logMessage1
}
