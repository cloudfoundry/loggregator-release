package integration_test

import (
	"encoding/json"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
)

var _ = Describe("Varz Endpoints", func() {
	var session *gexec.Session
	localIPAddress, _ := localip.LocalIP()

	BeforeSuite(func() {
		pathToMetronExecutable, err := gexec.Build("metron")
		Expect(err).ShouldNot(HaveOccurred())

		command := exec.Command(pathToMetronExecutable, "--configFile=fixtures/metron.json")

		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		// wait for server to be up
		Eventually(func() error {
			_, err := http.Get("http://" + localIPAddress + ":1234")
			return err
		}).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		session.Kill()
		gexec.CleanupBuildArtifacts()
	})

	Context("/varz", func() {

		var getVarzMessage = func() *instrumentation.VarzMessage {
			req, _ := http.NewRequest("GET", "http://"+localIPAddress+":1234/varz", nil)
			req.SetBasicAuth("admin", "admin")

			resp, _ := http.DefaultClient.Do(req)

			var message instrumentation.VarzMessage
			json.NewDecoder(resp.Body).Decode(&message)

			return &message
		}

		var getAgentListenerContext = func() *instrumentation.Context {
			message := getVarzMessage()

			for _, context := range message.Contexts {
				if context.Name == "agentListener" {
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
			agentListenerContext := getAgentListenerContext()
			Expect(agentListenerContext.Metrics[1].Name).To(Equal("receivedMessageCount"))
			expectedValue := agentListenerContext.Metrics[1].Value.(float64) + 1

			connection, _ := net.Dial("udp", "localhost:51160")
			connection.Write([]byte("test-data"))

			Eventually(func() interface{} {
				agentListenerContext = getAgentListenerContext()
				return agentListenerContext.Metrics[1].Value
			}).Should(Equal(expectedValue))
		})
	})

	Context("/healthz", func() {
		It("is ok", func() {
			resp, _ := http.Get("http://" + localIPAddress + ":1234/healthz")

			bodyString, _ := ioutil.ReadAll(resp.Body)
			Expect(string(bodyString)).To(Equal("ok"))
		})
	})

	Context("Message forwarding", func() {
		It("forwards messages to the specified UDP server", func(done Done) {
			defer close(done)
			testServer, _ := net.ListenPacket("udp", "localhost:3456")
			defer testServer.Close()

			connection, _ := net.Dial("udp", "localhost:51160")
			connection.Write([]byte("test-data"))

			readBuffer := make([]byte, 65535)
			readCount, _, _ := testServer.ReadFrom(readBuffer)
			readData := make([]byte, readCount)
			copy(readData, readBuffer[:readCount])

			Expect(readData).Should(BeEquivalentTo("test-data"))
		})
	})
})
