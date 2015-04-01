package integration_test

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("/varz endpoint", func() {
	var getVarzMessage = func() *instrumentation.VarzMessage {
		req, _ := http.NewRequest("GET", "http://"+localIPAddress+":1234/varz", nil)
		req.SetBasicAuth("admin", "admin")

		resp, _ := http.DefaultClient.Do(req)

		var message instrumentation.VarzMessage
		json.NewDecoder(resp.Body).Decode(&message)

		return &message
	}

	var getContext = func(name string) *instrumentation.Context {
		message := getVarzMessage()

		for _, context := range message.Contexts {
			if context.Name == name {
				return &context
			}
		}

		return nil
	}

	It("shows basic metrics", func() {
		message := getVarzMessage()

		Expect(message.Name).To(Equal("MetronAgent"))
		Expect(message.Tags).To(HaveKeyWithValue("ip", localIPAddress))
		Expect(message.NumGoRoutines).To(BeNumerically(">", 0))
		Expect(message.NumCpus).To(BeNumerically(">", 0))
		Expect(message.MemoryStats.BytesAllocatedHeap).To(BeNumerically(">", 0))
	})

	It("Increments metric counter when it receives a message", func() {
		agentListenerContext := getContext("legacyAgentListener")
		metric := getMetricFromContext(agentListenerContext, "receivedMessageCount")
		expectedValue := metric.Value.(float64) + 1

		connection, _ := net.Dial("udp", "localhost:51160")
		connection.Write([]byte("test-data"))

		Eventually(func() interface{} {
			agentListenerContext = getContext("legacyAgentListener")
			return getMetricFromContext(agentListenerContext, "receivedMessageCount").Value
		}).Should(BeNumerically(">=", expectedValue))
	})

	It("updates message aggregator metrics when it receives a message", func() {
		context := getContext("dropsondeUnmarshaller")
		metric := getMetricFromContext(context, "heartbeatReceived")
		expectedValue := metric.Value.(float64) + 1

		connection, _ := net.Dial("udp", "localhost:51161")

		message := basicHeartbeatMessage()
		connection.Write(message)

		Eventually(func() interface{} {
			context = getContext("dropsondeUnmarshaller")
			return getMetricFromContext(context, "heartbeatReceived").Value
		}).Should(BeNumerically(">=", expectedValue), "heartbeatReceived counter did not increment")
	})

	It("includes value metrics from sources", func() {
		connection, _ := net.Dial("udp", "localhost:51161")
		connection.Write(basicValueMessage())

		Eventually(func() *instrumentation.Context { return getContext("forwarder") }).ShouldNot(BeNil())

		metric := getMetricFromContext(getContext("forwarder"), "fake-origin-2.fake-metric-name")
		Expect(metric).ToNot(BeNil())
		Expect(metric.Name).To(Equal("fake-origin-2.fake-metric-name"))
		Expect(metric.Value).To(BeNumerically("==", 42))
		Expect(metric.Tags["component"]).To(Equal("test-component"))
	})

	It("includes counter event metrics from sources", func() {
		connection, _ := net.Dial("udp", "localhost:51161")
		connection.Write(basicCounterEventMessage())
		connection.Write(basicCounterEventMessage())

		Eventually(func() *instrumentation.Context { return getContext("forwarder") }).ShouldNot(BeNil())

		metric := getMetricFromContext(getContext("forwarder"), "fake-origin-2.fake-counter-event-name")
		Expect(metric.Name).To(Equal("fake-origin-2.fake-counter-event-name"))
		Expect(metric.Value).To(BeNumerically("==", 2))
		Expect(metric.Tags["component"]).To(Equal("test-component"))
	})
})

var _ = Describe("/healthz", func() {
	It("is ok", func() {
		resp, _ := http.Get("http://" + localIPAddress + ":1234/healthz")

		bodyString, _ := ioutil.ReadAll(resp.Body)
		Expect(string(bodyString)).To(Equal("ok"))
	})
})
