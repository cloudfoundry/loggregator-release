package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"time"
)

var _ = BeforeSuite(func() {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	command := exec.Command(pathToMetronExecutable, "--configFile=fixtures/metron.json")

	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ShouldNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	// wait for server to be up
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":1234")
		return err
	}).ShouldNot(HaveOccurred())

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
})

var session *gexec.Session
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string

var _ = AfterSuite(func() {
	session.Kill()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})

var _ = BeforeEach(func() {
	adapter := etcdRunner.Adapter()
	adapter.Disconnect()
	etcdRunner.Reset()
	adapter.Connect()
})

var _ = Describe("Varz Endpoints", func() {

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
				if context.Name == "legacyAgentListener" {
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

	Context("Legacy message forwarding", func() {
		It("forwards messages to a healthy loggregator server", func(done Done) {
			defer close(done)
			testServer, _ := net.ListenPacket("udp", "localhost:3456")
			defer testServer.Close()

			node := storeadapter.StoreNode{
				Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0",
				Value: []byte("localhost"),
			}
			adapter := etcdRunner.Adapter()
			adapter.Create(node)

			connection, _ := net.Dial("udp", "localhost:51160")

			stopWrite := make(chan struct{})
			defer close(stopWrite)
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					connection.Write([]byte("test-data"))

					select {
					case <-stopWrite:
						return
					case <-ticker.C:
					}
				}
			}()

			readBuffer := make([]byte, 65535)
			readCount, _, _ := testServer.ReadFrom(readBuffer)
			readData := make([]byte, readCount)
			copy(readData, readBuffer[:readCount])

			Expect(readData).Should(BeEquivalentTo("test-data"))
		})
	})

	Context("Dropsonde message forwarding", func() {
		It("forwards hmac signed messages to a healthy loggregator server", func(done Done) {
			defer close(done)

			messageString := "test-data"

			mac := hmac.New(sha256.New, []byte("shared_secret"))
			mac.Write([]byte(messageString))
			expectedMAC := mac.Sum(nil)

			testServer, _ := net.ListenPacket("udp", "localhost:3457")
			defer testServer.Close()

			node := storeadapter.StoreNode{
				Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller/0",
				Value: []byte("localhost"),
			}
			adapter := etcdRunner.Adapter()
			adapter.Create(node)

			connection, _ := net.Dial("udp", "localhost:51161")

			stopWrite := make(chan struct{})
			defer close(stopWrite)
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					connection.Write([]byte(messageString))

					select {
					case <-stopWrite:
						return
					case <-ticker.C:
					}
				}
			}()

			readBuffer := make([]byte, 65535)
			readCount, _, _ := testServer.ReadFrom(readBuffer)
			readData := make([]byte, readCount)
			copy(readData, readBuffer[:readCount])

			signature := readData[:32]
			messageData := readData[32:]
			Expect(messageData).Should(BeEquivalentTo(messageString))
			Expect(hmac.Equal(signature, expectedMAC)).To(BeTrue())
		})
	})
})
