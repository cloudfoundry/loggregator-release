package inputrouter_test

import (
	"trafficcontroller/inputrouter"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"trafficcontroller/hasher"
)

var _ = Describe("InputRouter", func() {
	var logger = gosteno.NewLogger("TestLogger")

	var (
		dataChan1, dataChan2 <-chan []byte
		listenerPort1        = "9998"
		listenerPort2        = "9997"
		listener1, listener2 agentlistener.AgentListener
		h                    hasher.Hasher
		r                    *inputrouter.Router
		logEmitter           *emitter.LoggregatorEmitter
	)

	BeforeSuite(func() {
		listener1, dataChan1 = agentlistener.NewAgentListener("localhost:"+listenerPort1, logger)
		go listener1.Start()

		listener2, dataChan2 = agentlistener.NewAgentListener("localhost:"+listenerPort2, logger)
		go listener2.Start()

		verifyListenerStarted(dataChan1, listenerPort1)
		verifyListenerStarted(dataChan2, listenerPort2)
	})

	JustBeforeEach(func() {
		r, _ = inputrouter.NewRouter("localhost:3551", h, cfcomponent.Config{}, logger)

		go r.Start(logger)
		logEmitter, _ = emitter.NewEmitter("localhost:3551", "ROUTER", "42", "secret", logger)

		var received []byte
		Eventually(func() []byte {
			logEmitter.Emit("test message appId", "test message")
			received = <-dataChan1
			return received
		}).ShouldNot(BeEmpty())
	})

	AfterEach(func() {
		r.Stop()
	})

	AfterSuite(func() {
		listener1.Stop()
		listener2.Stop()
	})

	Context("with one Loggregator", func() {
		BeforeEach(func() {
			loggregatorServers := []string{"localhost:" + listenerPort1}
			h = hasher.NewHasher(loggregatorServers)
		})

		It("routes messages", func() {
			logEmitter.Emit("my_awesome_app", "Hello World")

			var received []byte
			Eventually(dataChan1).Should(Receive(&received))

			receivedEnvelope := &logmessage.LogEnvelope{}
			proto.Unmarshal(received, receivedEnvelope)

			Expect(receivedEnvelope.GetLogMessage().GetAppId()).To(Equal("my_awesome_app"))
			Expect(string(receivedEnvelope.GetLogMessage().GetMessage())).To(Equal("Hello World"))
		})
	})

	Context("with two Loggregators", func() {
		BeforeEach(func() {
			loggregatorServers := []string{"localhost:" + listenerPort1, "localhost:" + listenerPort2}
			h = hasher.NewHasher(loggregatorServers)
		})

		It("routes messages to the correct Loggregator", func() {
			logEmitter.Emit("2", "My message")

			var receivedData []byte
			Eventually(dataChan1).Should(Receive(&receivedData))

			receivedEnvelope := &logmessage.LogEnvelope{}
			proto.Unmarshal(receivedData, receivedEnvelope)

			Expect(string(receivedEnvelope.GetLogMessage().GetMessage())).To(Equal("My message"))

			logEmitter.Emit("1", "Another message")

			Eventually(dataChan2).Should(Receive(&receivedData))
			receivedEnvelope = &logmessage.LogEnvelope{}
			proto.Unmarshal(receivedData, receivedEnvelope)

			Expect(string(receivedEnvelope.GetLogMessage().GetMessage())).To(Equal("Another message"))
		})
	})

	Context("with bad messages", func() {
		BeforeEach(func() {
			loggregatorServers := []string{"localhost:" + listenerPort1}
			h = hasher.NewHasher(loggregatorServers)
		})

		It("only routes properly formatted messages", func() {
			lc := loggregatorclient.NewLoggregatorClient("localhost:3551", logger, loggregatorclient.DefaultBufferSize)
			lc.Send([]byte("This is poorly formatted"))

			logEmitter.Emit("my_awesome_app", "Hello World")

			var received []byte
			Eventually(dataChan1).Should(Receive(&received))
			receivedEnvelope := &logmessage.LogEnvelope{}
			proto.Unmarshal(received, receivedEnvelope)

			Expect(string(receivedEnvelope.GetLogMessage().GetMessage())).To(Equal("Hello World"))
		})
	})

	Context("with envelopes", func() {
		BeforeEach(func() {
			loggregatorServers := []string{"localhost:" + listenerPort1}
			h = hasher.NewHasher(loggregatorServers)
		})

		It("routes based on routing key in envelope", func() {
			logEmitter.Emit("my_awesome_app", "Hello World")

			var received []byte
			Eventually(dataChan1).Should(Receive(&received))
			receivedEnvelope := &logmessage.LogEnvelope{}
			proto.Unmarshal(received, receivedEnvelope)

			Expect(string(receivedEnvelope.GetLogMessage().GetMessage())).To(Equal("Hello World"))
		})
	})
})
