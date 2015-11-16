package truncatingbuffer_test

import (
	"truncatingbuffer"
	"fmt"
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

type FakeContext struct {}

func(FakeContext) EventAllowed(events.Envelope_EventType) bool {
	return true
}

func(FakeContext) Identifier() string {
	return "test-sink-name"
}

func(FakeContext) DropsondeOrigin() string {
	return "doppler"
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
	logMsgCount int
	truncateChan chan struct {}
	FakeContext
}

func NewLogFilteredContext(truncateChan chan struct {}) *LogFilteredContext {
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
	var buffer *truncatingbuffer.TruncatingBuffer
	var context truncatingbuffer.BufferContext

	BeforeEach(func() {
		inMessageChan = make(chan *events.Envelope)
		stopChannel = make(chan struct{})
		context = &FakeContext{}
	})

	JustBeforeEach(func() {
		buffer = truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, context, loggertesthelper.Logger(), stopChannel)
	})

	AfterEach(func() {
		if inMessageChan != nil {
			close(inMessageChan)
		}
	})

	It("panics if buffer size is less than 3", func() {
		Expect(func() {
			truncatingbuffer.NewTruncatingBuffer(inMessageChan, 2,context, loggertesthelper.Logger(), nil)
		}).To(Panic())
	})

	It("panics if context is nil", func() {
		Expect(func() {
			truncatingbuffer.NewTruncatingBuffer(inMessageChan, 3, nil, loggertesthelper.Logger(), nil)
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
			var fakeEventEmitter *fake.FakeEventEmitter

			BeforeEach(func() {
				fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
				sender := metric_sender.NewMetricSender(fakeEventEmitter)
				batcher := metricbatcher.New(sender, 100*time.Millisecond)

				metrics.Initialize(sender, batcher)
				fakeEventEmitter.Reset()
			})

			tracksDroppedMessages := func(itMsg string, delta, total int) {
				It(itMsg, func() {
					var logMessageNotification *events.Envelope

					Eventually(buffer.GetOutputChannel).Should(Receive(&logMessageNotification))
					Expect(logMessageNotification.GetEventType()).To(Equal(events.Envelope_LogMessage))
					Expect(logMessageNotification.GetLogMessage().GetMessage()).To(ContainSubstring(fmt.Sprintf("Log message output is too high. %d messages dropped (Total %d messages dropped) to test-sink-name.", delta, total)))

					var counterEventNotification *events.Envelope
					Eventually(buffer.GetOutputChannel).Should(Receive(&counterEventNotification))
					Expect(counterEventNotification.GetEventType()).To(Equal(events.Envelope_CounterEvent))
					counterEvent := counterEventNotification.GetCounterEvent()
					Expect(counterEvent.GetName()).To(Equal("TruncatingBuffer.DroppedMessages"))
					Expect(counterEvent.GetDelta()).To(BeEquivalentTo(delta))
					Expect(counterEvent.GetTotal()).To(BeEquivalentTo(total))
				})

				It("keeps track of dropped messages", func() {
					Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
					Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
						Name:  proto.String("TruncatingBuffer.totalDroppedMessages"),
						Delta: proto.Uint64(uint64(total)),
					}))
				})
			}

			Context("when the buffer fills once", func() {
				JustBeforeEach(func() {
					Expect(buffer.GetDroppedMessageCount()).To(BeZero())

					firstBuffer := buffer.GetOutputChannel()
					sendLogMessages("message 1", inMessageChan)
					sendLogMessages("message 2", inMessageChan)
					sendLogMessages("message 3", inMessageChan)
					sendLogMessages("message 4", inMessageChan)
					Eventually(buffer.GetOutputChannel).ShouldNot(Equal(firstBuffer))
				})

				tracksDroppedMessages("drops all the messages", 3, 3)

				Context("when the buffer fills multiple times", func() {
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

						Eventually(buffer.GetOutputChannel).ShouldNot(Equal(outputChannel))
					})

					Context("no event is read", func() {
						BeforeEach(func() {
							receiveEvents = 0
							sendLog = 1
						})

						tracksDroppedMessages("drops immediately", 1, 4)
					})

					Context("and the TB log event is read", func() {
						BeforeEach(func() {
							receiveEvents = 1
							sendLog = 2
						})

						tracksDroppedMessages("has 1 slot filled", 2, 5)
					})

					Context("and the TB log and counter event is read", func() {
						BeforeEach(func() {
							receiveEvents = 2
							sendLog = 3
						})

						tracksDroppedMessages("has an empty buffer", 3, 6)
					})
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
