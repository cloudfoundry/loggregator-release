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

		connection, _ := net.Dial("udp4", "localhost:51160")
		connection.Write([]byte("test-data"))

		Eventually(func() interface{} {
			agentListenerContext = getContext("legacyAgentListener")
			return getMetricFromContext(agentListenerContext, "receivedMessageCount").Value
		}).Should(BeNumerically(">=", expectedValue))
	})
})

var _ = Describe("/healthz", func() {
	It("is ok", func() {
		resp, _ := http.Get("http://" + localIPAddress + ":1234/healthz")

		bodyString, _ := ioutil.ReadAll(resp.Body)
		Expect(string(bodyString)).To(Equal("ok"))
	})
})
