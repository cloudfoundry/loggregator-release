package sinkserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinkserver"
	"sync"
)

type fakeSinkManager struct {
	sync.RWMutex
	receivedMessages []*logmessage.Message
	receivedDrains   [][]string
}

func (f *fakeSinkManager) SendTo(appId string, receivedMessage *logmessage.Message) {
	f.Lock()
	defer f.Unlock()
	f.receivedMessages = append(f.receivedMessages, receivedMessage)
}

func (f *fakeSinkManager) ManageSyslogSinks(appId string, syslogSinkUrls []string) {
	f.Lock()
	defer f.Unlock()
	f.receivedDrains = append(f.receivedDrains, syslogSinkUrls)
}

func (f *fakeSinkManager) received() []*logmessage.Message {
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
		fakeManager = &fakeSinkManager{receivedMessages: make([]*logmessage.Message, 0), receivedDrains: make([][]string, 0)}
		messageRouter = sinkserver.NewMessageRouter(fakeManager, loggertesthelper.Logger())
	})

	Describe("Start", func() {
		Context("With an incoming message", func() {
			var incomingLogChan chan *logmessage.Message
			BeforeEach(func() {
				incomingLogChan = make(chan *logmessage.Message)
				go messageRouter.Start(incomingLogChan)
			})

			AfterEach(func() {
				messageRouter.Stop()
			})

			It("sends the message to the sink manager if it is an app message", func() {
				message := NewMessage("testMessage", "app")
				incomingLogChan <- message
				Eventually(fakeManager.received).Should(HaveLen(1))
				Expect(fakeManager.received()[0].GetLogMessage()).To(Equal(message.GetLogMessage()))
			})

			It("doesn't send drain URLs not from Warden", func() {
				message := NewMessage("testMessage", "app")
				dea := "DEA"
				message.GetLogMessage().SourceName = &dea
				drains := []string{"drainurl"}

				message.GetLogMessage().DrainUrls = drains
				incomingLogChan <- message
				Eventually(fakeManager.drains).ShouldNot(HaveLen(1))
			})
		})
	})

	Describe("Stop", func() {
		It("should return", func() {
			incomingLogChan := make(chan *logmessage.Message)
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
