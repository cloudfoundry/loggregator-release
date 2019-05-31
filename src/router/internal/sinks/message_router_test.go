package sinks_test

import (
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/router/internal/sinks"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Message Router", func() {

	var (
		fakeManagerA  *fakeSinkManager
		fakeManagerB  *fakeSinkManager
		messageRouter *sinks.MessageRouter
	)

	BeforeEach(func() {
		fakeManagerA = &fakeSinkManager{
			receivedMessages: make([]*events.Envelope, 0),
			receivedDrains:   make([][]string, 0),
		}

		fakeManagerB = &fakeSinkManager{
			receivedMessages: make([]*events.Envelope, 0),
			receivedDrains:   make([][]string, 0),
		}

		messageRouter = sinks.NewMessageRouter(fakeManagerA, fakeManagerB)
	})

	Describe("Start", func() {
		Context("with an incoming message", func() {
			var incoming *diodes.ManyToOneEnvelope
			BeforeEach(func() {
				incoming = diodes.NewManyToOneEnvelope(5, nil)
				go messageRouter.Start(incoming)
			})

			It("sends the message to each sender if it is an app message", func() {
				message, _ := wrap(newLogMessage(events.LogMessage_OUT, "testMessage", "app", "App"), "origin")
				incoming.Set(message)
				Eventually(fakeManagerA.received).Should(HaveLen(1))
				Eventually(fakeManagerB.received).Should(HaveLen(1))
				Expect(fakeManagerA.received()[0].GetLogMessage()).To(Equal(message.GetLogMessage()))
				Expect(fakeManagerB.received()[0].GetLogMessage()).To(Equal(message.GetLogMessage()))
			})
		})
	})
})

var errorMissingOrigin = errors.New("Event not emitted due to missing origin information")
var errorUnknownEventType = errors.New("Cannot create envelope for unknown event type")

func wrap(event events.Event, origin string) (*events.Envelope, error) {
	if origin == "" {
		return nil, errorMissingOrigin
	}

	envelope := &events.Envelope{
		Origin:    proto.String(origin),
		Timestamp: proto.Int64(time.Now().UnixNano()),
	}

	switch event := event.(type) {
	case *events.HttpStartStop:
		envelope.EventType = events.Envelope_HttpStartStop.Enum()
		envelope.HttpStartStop = event
	case *events.ValueMetric:
		envelope.EventType = events.Envelope_ValueMetric.Enum()
		envelope.ValueMetric = event
	case *events.CounterEvent:
		envelope.EventType = events.Envelope_CounterEvent.Enum()
		envelope.CounterEvent = event
	case *events.LogMessage:
		envelope.EventType = events.Envelope_LogMessage.Enum()
		envelope.LogMessage = event
	case *events.ContainerMetric:
		envelope.EventType = events.Envelope_ContainerMetric.Enum()
		envelope.ContainerMetric = event
	default:
		return nil, errorUnknownEventType
	}

	return envelope, nil
}

func newLogMessage(messageType events.LogMessage_MessageType, messageString, appId, sourceType string) *events.LogMessage {
	currentTime := time.Now()

	logMessage := &events.LogMessage{
		Message:     []byte(messageString),
		AppId:       &appId,
		MessageType: &messageType,
		SourceType:  proto.String(sourceType),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}

type fakeSinkManager struct {
	sync.RWMutex
	receivedMessages []*events.Envelope
	receivedDrains   [][]string
}

func (f *fakeSinkManager) SendTo(appId string, receivedMessage *events.Envelope) {
	f.Lock()
	defer f.Unlock()
	f.receivedMessages = append(f.receivedMessages, receivedMessage)
}

func (f *fakeSinkManager) ManageSyslogSinks(appId string, syslogSinkUrls []string) {
	f.Lock()
	defer f.Unlock()
	f.receivedDrains = append(f.receivedDrains, syslogSinkUrls)
}

func (f *fakeSinkManager) received() []*events.Envelope {
	f.RLock()
	defer f.RUnlock()
	return f.receivedMessages
}

func (f *fakeSinkManager) drains() [][]string {
	f.RLock()
	defer f.RUnlock()
	return f.receivedDrains
}
