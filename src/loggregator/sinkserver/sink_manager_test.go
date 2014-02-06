package sinkserver_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/iprange"
	"loggregator/sinkserver"
)

type ChannelSink struct {
	appId, identifier string
	logger            *gosteno.Logger
	channel           chan *logmessage.Message
}

func (c *ChannelSink) AppId() string                     { return c.appId }
func (c *ChannelSink) Run()                              {}
func (c *ChannelSink) Channel() chan *logmessage.Message { return c.channel }
func (c *ChannelSink) Identifier() string                { return c.identifier }
func (c *ChannelSink) Logger() *gosteno.Logger           { return c.logger }
func (c *ChannelSink) ShouldReceiveErrors() bool         { return true }
func (c *ChannelSink) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

var _ = Describe("SinkManager", func() {
	var sinkManager *sinkserver.SinkManager

	BeforeEach(func() {
		sinkManager = sinkserver.NewSinkManager(1, true, []iprange.IPRange{}, loggertesthelper.Logger())
		go sinkManager.Start()
	})

	Describe("SendTo", func() {
		It("should send to all known sinks", func(done Done) {

			client1ReceivedChan := make(chan *logmessage.Message)
			client2ReceivedChan := make(chan *logmessage.Message)
			sink1 := &ChannelSink{appId: "myApp",
				identifier: "myAppChan1",
				logger:     loggertesthelper.Logger(),
				channel:    client1ReceivedChan,
			}
			sink2 := &ChannelSink{appId: "myApp",
				identifier: "myAppChan2",
				logger:     loggertesthelper.Logger(),
				channel:    client2ReceivedChan,
			}

			sinkManager.RegisterSink(sink1)
			sinkManager.RegisterSink(sink2)

			expectedMessageString := "Some Data"
			expectedMessage := NewMessage(expectedMessageString, "myApp")
			go sinkManager.SendTo("myApp", expectedMessage)

			receiveMessage := <-client1ReceivedChan
			Expect(receiveMessage).To(Equal(expectedMessage))
			receiveMessage = <-client2ReceivedChan
			Expect(receiveMessage).To(Equal(expectedMessage))

			close(done)
		})
	})
})
