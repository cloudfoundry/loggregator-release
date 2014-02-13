package sinkmanager_test

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/iprange"
	"loggregator/sinkserver/sinkmanager"
	"sync"
	"loggregator/sinkserver/blacklist"
)

type ChannelSink struct {
	sync.RWMutex
	appId, identifier string
	received           []*logmessage.Message
}

func (c *ChannelSink) AppId() string                     { return c.appId }
func (c *ChannelSink) Run(msgChan <-chan *logmessage.Message)    {
	for msg := range(msgChan) {
		c.Lock()
		c.received = append(c.received, msg)
		c.Unlock()
	}
}

func (c *ChannelSink) Received() []*logmessage.Message {
	c.RLock()
	defer c.RUnlock()
	data := make([]*logmessage.Message,len(c.received))
	copy(data, c.received)
	return data
}

func (c *ChannelSink) Identifier() string                { return c.identifier }
func (c *ChannelSink) ShouldReceiveErrors() bool         { return true }
func (c *ChannelSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

var _ = Describe("SinkManager", func() {
	var sinkManager *sinkmanager.SinkManager
	var blackListManager *blacklist.URLBlacklistManager

	BeforeEach(func() {
		blackListManager = blacklist.New([]iprange.IPRange{})
		sinkManager = sinkmanager.NewSinkManager(1, true, blackListManager, loggertesthelper.Logger())
		go sinkManager.Start()
	})

	Describe("SendSyslogErrorToLoggregator", func() {
		It("should listen and broadcast error messages", func() {

			sink := &ChannelSink{appId: "myApp",
				identifier: "myAppChan1",
			}
			sinkManager.RegisterSink(sink)
			sinkManager.SendSyslogErrorToLoggregator("error msg", "myApp")

			Eventually(sink.Received).Should(HaveLen(1))

			errorMsg := sink.Received()[0]
			Expect(string(errorMsg.GetLogMessage().GetMessage())).To(Equal("error msg"))

		})
	})

	Describe("SendTo", func() {
		It("should send to all known sinks", func() {

			sink1 := &ChannelSink{appId: "myApp",
				identifier: "myAppChan1",
			}
			sink2 := &ChannelSink{appId: "myApp",
				identifier: "myAppChan2",
			}

			sinkManager.RegisterSink(sink1)
			sinkManager.RegisterSink(sink2)

			expectedMessageString := "Some Data"
			expectedMessage := NewMessage(expectedMessageString, "myApp")
			go sinkManager.SendTo("myApp", expectedMessage)

			Eventually(sink1.Received).Should(HaveLen(1))
			Eventually(sink2.Received).Should(HaveLen(1))
			Expect(sink1.Received()[0]).To(Equal(expectedMessage))
			Expect(sink2.Received()[0]).To(Equal(expectedMessage))

		})

		It("should only send to sinks that match the appID", func(done Done) {

			sink1 := &ChannelSink{appId: "myApp1",
				identifier: "myAppChan1",
			}
			sink2 := &ChannelSink{appId: "myApp2",
				identifier: "myAppChan2",
			}

			sinkManager.RegisterSink(sink1)
			sinkManager.RegisterSink(sink2)

			expectedMessageString := "Some Data"
			expectedMessage := NewMessage(expectedMessageString, "myApp")
			go sinkManager.SendTo("myApp1", expectedMessage)

			Eventually(sink1.Received).Should(HaveLen(1))
			Expect(sink1.Received()[0]).To(Equal(expectedMessage))
			Expect(sink2.Received()).To(BeEmpty())

			close(done)
		})
	})

	Describe("ManageSyslogSinks", func(){

		It("should send new sinks to the app store", func() {

		})

		It("not allow duplicate sinks to be registered", func(){
			activeSyslogSinksCounter := sinkManager.Metrics.SyslogSinks

			sinkManager.ManageSyslogSinks("appid",[]string{"http://10.10.123.1"})

			Expect(sinkManager.Metrics.SyslogSinks).To(Equal(activeSyslogSinksCounter+1))

			sinkManager.ManageSyslogSinks("appid",[]string{"http://10.10.123.1"})

			Expect(sinkManager.Metrics.SyslogSinks).To(Equal(activeSyslogSinksCounter+1))
		})
	})
})
