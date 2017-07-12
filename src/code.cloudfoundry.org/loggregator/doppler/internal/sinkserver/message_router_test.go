package sinkserver_test

import (
	"sync"

	"code.cloudfoundry.org/loggregator/diodes"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

var _ = Describe("Message Router", func() {

	var (
		fakeManagerA  *fakeSinkManager
		fakeManagerB  *fakeSinkManager
		messageRouter *sinkserver.MessageRouter
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

		messageRouter = sinkserver.NewMessageRouter(fakeManagerA, fakeManagerB)
	})

	Describe("Start", func() {
		Context("with an incoming message", func() {
			var incoming *diodes.ManyToOneEnvelope
			BeforeEach(func() {
				incoming = diodes.NewManyToOneEnvelope(5, nil)
				go messageRouter.Start(incoming)
			})

			It("sends the message to each sender if it is an app message", func() {
				message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "testMessage", "app", "App"), "origin")
				incoming.Set(message)
				Eventually(fakeManagerA.received).Should(HaveLen(1))
				Eventually(fakeManagerB.received).Should(HaveLen(1))
				Expect(fakeManagerA.received()[0].GetLogMessage()).To(Equal(message.GetLogMessage()))
				Expect(fakeManagerB.received()[0].GetLogMessage()).To(Equal(message.GetLogMessage()))
			})
		})
	})
})
