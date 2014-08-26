package sinkserver_test

import (
	"doppler/envelopewrapper"
	"doppler/sinkserver"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeSinkManager struct {
	sync.RWMutex
	receivedMessages []*envelopewrapper.WrappedEnvelope
	receivedDrains   [][]string
}

func (f *fakeSinkManager) SendTo(appId string, receivedMessage *envelopewrapper.WrappedEnvelope) {
	f.Lock()
	defer f.Unlock()
	f.receivedMessages = append(f.receivedMessages, receivedMessage)
}

func (f *fakeSinkManager) ManageSyslogSinks(appId string, syslogSinkUrls []string) {
	f.Lock()
	defer f.Unlock()
	f.receivedDrains = append(f.receivedDrains, syslogSinkUrls)
}

func (f *fakeSinkManager) received() []*envelopewrapper.WrappedEnvelope {
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

	var fakeManager *fakeSinkManager
	var messageRouter *sinkserver.MessageRouter

	BeforeEach(func() {
		fakeManager = &fakeSinkManager{receivedMessages: make([]*envelopewrapper.WrappedEnvelope, 0), receivedDrains: make([][]string, 0)}
		messageRouter = sinkserver.NewMessageRouter(fakeManager, loggertesthelper.Logger())
	})

	Describe("Start", func() {
		Context("With an incoming message", func() {
			var incomingLogChan chan *envelopewrapper.WrappedEnvelope
			BeforeEach(func() {
				incomingLogChan = make(chan *envelopewrapper.WrappedEnvelope)
				go messageRouter.Start(incomingLogChan)
			})

			AfterEach(func() {
				messageRouter.Stop()
			})

			It("sends the message to the sink manager if it is an app message", func() {
				message, _ := envelopewrapper.WrapEvent(factories.NewLogMessage(events.LogMessage_OUT, "testMessage", "app", "App"), "origin")
				incomingLogChan <- message
				Eventually(fakeManager.received).Should(HaveLen(1))
				Expect(fakeManager.received()[0].Envelope.GetLogMessage()).To(Equal(message.Envelope.GetLogMessage()))
			})
		})
	})

	Describe("Stop", func() {
		It("should return", func() {
			incomingLogChan := make(chan *envelopewrapper.WrappedEnvelope)
			done := make(chan struct{})
			go func() {
				messageRouter.Start(incomingLogChan)
				close(done)
			}()
			messageRouter.Stop()
			Eventually(done).Should(BeClosed())
		})

	})

})
