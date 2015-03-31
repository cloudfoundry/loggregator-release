package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gunk/natsrunner"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const natsPort = 24484

var session *gexec.Session
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string

var _ = BeforeSuite(func() {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json", "--debug")

	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ShouldNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	// wait for server to be up
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":1234")
		return err
	}, 3).ShouldNot(HaveOccurred())

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
})

var _ = AfterSuite(func() {
	session.Kill().Wait()
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})

var _ = Describe("Metron", func() {
	Context("collector registration", func() {
		It("registers itself with the collector", func() {
			natsRunner := natsrunner.NewNATSRunner(natsPort)
			natsRunner.Start()
			defer natsRunner.Stop()

			messageChan := make(chan []byte)
			natsClient := natsRunner.MessageBus
			natsClient.Subscribe(collectorregistrar.AnnounceComponentMessageSubject, func(msg *nats.Msg) {
				messageChan <- msg.Data
			})

			Eventually(messageChan).Should(Receive(MatchRegexp(`^\{"type":"MetronAgent","index":42,"host":"[^:]*:1234","uuid":"42-","credentials":\["admin","admin"\]\}$`)))
		})
	})

	Context("/varz endpoint", func() {
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

	Context("/healthz", func() {
		It("is ok", func() {
			resp, _ := http.Get("http://" + localIPAddress + ":1234/healthz")

			bodyString, _ := ioutil.ReadAll(resp.Body)
			Expect(string(bodyString)).To(Equal("ok"))
		})
	})

	Context("Legacy message forwarding", func() {
		It("converts to events message format and forwards to doppler", func(done Done) {
			defer close(done)

			currentTime := time.Now()
			marshalledLegacyMessage := legacyLogMessage(123, "BLAH", currentTime)
			marshalledEventsMessage := eventsLogMessage(123, "BLAH", currentTime)

			testServer, _ := net.ListenPacket("udp", "localhost:3457")
			defer testServer.Close()

			node := storeadapter.StoreNode{
				Key:   "/healthstatus/doppler/z1/0",
				Value: []byte("localhost"),
			}
			adapter := etcdRunner.Adapter()
			adapter.Create(node)

			connection, _ := net.Dial("udp", "localhost:51160")
			connection.Write(marshalledLegacyMessage)

			readBuffer := make([]byte, 65535)
			messageChan := make(chan []byte, 1000)

			go func() {
				for {
					readCount, _, _ := testServer.ReadFrom(readBuffer)
					readData := make([]byte, readCount)
					copy(readData, readBuffer[:readCount])

					if len(readData) > 32 {
						messageChan <- readData[32:]
					}
				}
			}()

			stopWrite := make(chan struct{})
			defer close(stopWrite)
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					connection.Write(marshalledLegacyMessage)

					select {
					case <-stopWrite:
						return
					case <-ticker.C:
					}
				}
			}()

			Eventually(messageChan).Should(Receive(Equal(marshalledEventsMessage)))
		})
	})

	Context("Dropsonde message forwarding", func() {
		var testDoppler net.PacketConn

		BeforeEach(func() {
			testDoppler, _ = net.ListenPacket("udp", "localhost:3457")

			node := storeadapter.StoreNode{
				Key:   "/healthstatus/doppler/z1/0",
				Value: []byte("localhost"),
			}

			adapter := etcdRunner.Adapter()
			adapter.Create(node)
			adapter.Disconnect()
		})

		AfterEach(func() {
			testDoppler.Close()
		})

		It("forwards hmac signed messages to a healthy doppler server", func(done Done) {
			type signedMessage struct {
				signature []byte
				message   []byte
			}

			defer close(done)

			originalMessage := basicHeartbeatMessage()

			mac := hmac.New(sha256.New, []byte("shared_secret"))
			mac.Write(originalMessage)
			signature := mac.Sum(nil)

			metronInput, _ := net.Dial("udp", "localhost:51161")

			messageChan := make(chan signedMessage, 1000)

			stopTheWorld := make(chan struct{})
			defer close(stopTheWorld)

			readFromDoppler := func() {
				gotSignedMessage := func(readData []byte) bool {
					return len(readData) > len(signature)
				}

				readBuffer := make([]byte, 65535)

				for {
					readCount, _, _ := testDoppler.ReadFrom(readBuffer)
					readData := make([]byte, readCount)
					copy(readData, readBuffer[:readCount])

					if gotSignedMessage(readData) {
						messageChan <- signedMessage{signature: readData[:len(signature)], message: readData[len(signature):]}
					}

					select {
					case <-stopTheWorld:
						return
					default:
					}
				}
			}

			go readFromDoppler()

			writeToMetron := func() {
				ticker := time.NewTicker(10 * time.Millisecond)

				for {
					metronInput.Write(originalMessage)

					select {
					case <-stopTheWorld:
						ticker.Stop()
						return
					case <-ticker.C:
					}
				}
			}

			go writeToMetron()

			Eventually(messageChan).Should(Receive(Equal(signedMessage{signature: signature, message: originalMessage})))
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

func legacyLogMessage(appID int, message string, timestamp time.Time) []byte {
	envelope := &logmessage.LogEnvelope{
		RoutingKey: proto.String("fake-routing-key"),
		Signature:  []byte{1, 2, 3},
		LogMessage: &logmessage.LogMessage{
			Message:     []byte(message),
			MessageType: logmessage.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(timestamp.UnixNano()),
			AppId:       proto.String(string(appID)),
			SourceName:  proto.String("fake-source-id"),
			SourceId:    proto.String("fake-source-id"),
		},
	}

	bytes, _ := proto.Marshal(envelope)
	return bytes
}

func eventsLogMessage(appID int, message string, timestamp time.Time) []byte {
	envelope := &events.Envelope{
		Origin:    proto.String("legacy"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        []byte(message),
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(timestamp.UnixNano()),
			AppId:          proto.String(string(appID)),
			SourceType:     proto.String("fake-source-id"),
			SourceInstance: proto.String("fake-source-id"),
		},
	}

	bytes, _ := proto.Marshal(envelope)
	return bytes
}
