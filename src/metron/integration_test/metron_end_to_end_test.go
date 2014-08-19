package integration_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
			Expect(agentListenerContext.Metrics[1].Name).To(Equal("receivedMessageCount"))
			expectedValue := agentListenerContext.Metrics[1].Value.(float64) + 1

			connection, _ := net.Dial("udp", "localhost:51160")
			connection.Write([]byte("test-data"))

			Eventually(func() interface{} {
				agentListenerContext = getContext("legacyAgentListener")
				return agentListenerContext.Metrics[1].Value
			}).Should(Equal(expectedValue))
		})

		It("updates message aggregator metrics when it receives a message", func() {
			context := getContext("MessageAggregator")
			Expect(context.Metrics[3].Name).To(Equal("uncategorizedEvents"))
			expectedValue := context.Metrics[3].Value.(float64) + 1

			connection, _ := net.Dial("udp", "localhost:51161")

			message := basicHeartbeatMessage()
			connection.Write(message)

			Eventually(func() interface{} {
				context = getContext("MessageAggregator")
				return context.Metrics[3].Value
			}).Should(Equal(expectedValue), "uncategorizedEvents counter did not increment")
		})

		It("includes value metrics from sources", func() {
			connection, _ := net.Dial("udp", "localhost:51161")
			connection.Write(basicValueMessage())

			Eventually(func() *instrumentation.Context { return getContext("forwarder") }).ShouldNot(BeNil())

			metric := getMetricFromContext(getContext("forwarder"), "fake-origin-2.fake-metric-name")
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
				Key:   "/healthstatus/loggregator/z1/loggregator_trafficcontroller/0",
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

			originalMessage := basicHeartbeatMessage()

			mac := hmac.New(sha256.New, []byte("shared_secret"))
			mac.Write(originalMessage)
			expectedMAC := mac.Sum(nil)

			testServer, _ := net.ListenPacket("udp", "localhost:3457")
			defer testServer.Close()

			node := storeadapter.StoreNode{
				Key:   "/healthstatus/loggregator/z1/loggregator_trafficcontroller/0",
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
					connection.Write(originalMessage)

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
			Expect(messageData).Should(BeEquivalentTo(originalMessage))
			Expect(hmac.Equal(signature, expectedMAC)).To(BeTrue())
		})
	})
})

func basicHeartbeatMessage() []byte {
	peerType := events.PeerType_Server
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-1"),
		EventType: events.Envelope_Heartbeat.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp: proto.Int64(1),
			RequestId: &events.UUID{
				Low:  proto.Uint64(0),
				High: proto.Uint64(1),
			},
			PeerType:      &peerType,
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("fake-uri-1"),
			RemoteAddress: proto.String("fake-remote-addr-1"),
			UserAgent:     proto.String("fake-user-agent-1"),
			ParentRequestId: &events.UUID{
				Low:  proto.Uint64(2),
				High: proto.Uint64(3),
			},
			ApplicationId: &events.UUID{
				Low:  proto.Uint64(4),
				High: proto.Uint64(5),
			},
			InstanceIndex: proto.Int32(6),
			InstanceId:    proto.String("fake-instance-id-1"),
		},
	})

	return message
}

func basicValueMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	})

	return message
}

func basicCounterEventMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-counter-event-name"),
			Delta: proto.Uint64(1),
		},
	})

	return message
}

func getMetricFromContext(context *instrumentation.Context, name string) *instrumentation.Metric {
	for _, metric := range context.Metrics {
		if metric.Name == name {
			return &metric
		}
	}
	return nil
}
