package listeners_test

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/apoydence/eachers/testhelpers"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/nu7hatch/gouuid"

	"doppler/config"
	"doppler/listeners"
	"plumbing"
)

var _ = Describe("TCPlistener", func() {
	var (
		listener          *listeners.TCPListener
		envelopeChan      chan *events.Envelope
		tlsListenerConfig *config.TLSListenerConfig
		tlsClientConfig   *tls.Config
		mockBatcher       *mockBatcher
		mockChainer       *mockBatchCounterChainer
		deadline          time.Duration
	)

	BeforeEach(func() {
		mockBatcher = newMockBatcher()
		mockChainer = newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)
		deadline = 500 * time.Millisecond

		tlsListenerConfig = &config.TLSListenerConfig{
			CertFile: "fixtures/server.crt",
			KeyFile:  "fixtures/server.key",
			CAFile:   "fixtures/loggregator-ca.crt",
		}

		var err error
		tlsClientConfig, err = plumbing.NewTLSConfig(
			"fixtures/client.crt",
			"fixtures/client.key",
			"fixtures/loggregator-ca.crt",
			"doppler",
		)
		Expect(err).NotTo(HaveOccurred())

		envelopeChan = make(chan *events.Envelope)
	})

	JustBeforeEach(func() {
		var err error
		listener, err = listeners.NewTCPListener(
			"aname",
			"127.0.0.1:1234",
			tlsListenerConfig,
			envelopeChan,
			mockBatcher,
			deadline,
			loggertesthelper.Logger(),
		)
		Expect(err).NotTo(HaveOccurred())
		go listener.Start()

		// wait for the listener to start up
		openTCPConnection("127.0.0.1:1234", tlsClientConfig).Close()
	})

	AfterEach(func() {
		listener.Stop()
	})

	Context("with TLS disabled", func() {
		BeforeEach(func() {
			tlsListenerConfig = nil
			tlsClientConfig = nil
		})

		DescribeTable("supported message types", func(eventType events.Envelope_EventType) {
			envelope := createEnvelope(events.Envelope_EventType(eventType))
			conn := openTCPConnection(listener.Address(), tlsClientConfig)

			err := send(conn, envelope)
			Expect(err).ToNot(HaveOccurred())

			Eventually(envelopeChan).Should(Receive(Equal(envelope)))
			conn.Close()
		},
			Entry(events.Envelope_LogMessage.String(), events.Envelope_LogMessage),
			Entry(events.Envelope_HttpStartStop.String(), events.Envelope_HttpStartStop),
			Entry(events.Envelope_Error.String(), events.Envelope_Error),
			Entry(events.Envelope_CounterEvent.String(), events.Envelope_CounterEvent),
			Entry(events.Envelope_ValueMetric.String(), events.Envelope_ValueMetric),
			Entry(events.Envelope_ContainerMetric.String(), events.Envelope_ContainerMetric),
		)
	})

	Context("with TLS is enabled", func() {
		Context("with invalid client configuration", func() {
			JustBeforeEach(func() {
				conn := openTCPConnection(listener.Address(), tlsClientConfig)
				conn.Close()
			})

			Context("without a CA file", func() {
				It("fails", func() {
					tlsClientConfig, err := plumbing.NewTLSConfig(
						"fixtures/client.crt",
						"fixtures/client.key",
						"",
						"doppler",
					)
					Expect(err).NotTo(HaveOccurred())

					_, err = tls.Dial("tcp", listener.Address(), tlsClientConfig)
					Expect(err).To(MatchError("x509: certificate signed by unknown authority"))
				})
			})

			Context("without a server name", func() {
				It("fails", func() {
					tlsClientConfig.ServerName = ""
					_, err := tls.Dial("tcp", listener.Address(), tlsClientConfig)
					Expect(err).To(MatchError("x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs"))
				})
			})
		})

		Describe("dropsonde metric emission", func() {
			DescribeTable("supported message types across multiple connections", func(eventType events.Envelope_EventType) {
				envelope1 := createEnvelope(events.Envelope_EventType(eventType))
				conn1 := openTCPConnection(listener.Address(), tlsClientConfig)

				envelope2 := createEnvelope(events.Envelope_EventType(eventType))
				conn2 := openTCPConnection(listener.Address(), tlsClientConfig)

				err := send(conn1, envelope1)
				Expect(err).ToNot(HaveOccurred())
				err = send(conn2, envelope2)
				Expect(err).ToNot(HaveOccurred())

				envelopes := readMessages(envelopeChan, 2)
				Expect(envelopes).To(ContainElement(envelope1))
				Expect(envelopes).To(ContainElement(envelope2))

				conn1.Close()
				conn2.Close()
			},
				Entry(events.Envelope_LogMessage.String(), events.Envelope_LogMessage),
				Entry(events.Envelope_HttpStartStop.String(), events.Envelope_HttpStartStop),
				Entry(events.Envelope_Error.String(), events.Envelope_Error),
				Entry(events.Envelope_CounterEvent.String(), events.Envelope_CounterEvent),
				Entry(events.Envelope_ValueMetric.String(), events.Envelope_ValueMetric),
				Entry(events.Envelope_ContainerMetric.String(), events.Envelope_ContainerMetric),
			)

			It("issues intended metrics", func() {
				envelope := createEnvelope(events.Envelope_LogMessage)
				conn := openTCPConnection(listener.Address(), tlsClientConfig)

				err := send(conn, envelope)
				Expect(err).ToNot(HaveOccurred())
				conn.Close()

				Eventually(mockBatcher.BatchCounterInput).Should(BeCalled(With("listeners.receivedEnvelopes")))
				Eventually(mockChainer.SetTagInput).Should(BeCalled(
					With("protocol", "tls"),
					With("event_type", "LogMessage"),
				))
				Eventually(mockChainer.IncrementCalled).Should(BeCalled())
				Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
					With("listeners.totalReceivedMessageCount"),
					With("aname.receivedMessageCount"),
				))
				Eventually(mockBatcher.BatchAddCounterInput).Should(BeCalled(
					With("aname.receivedByteCount", uint64(67)),
					With("listeners.totalReceivedByteCount", uint64(67)),
				))

				Eventually(envelopeChan).Should(Receive())
			})

			Describe("connection deadlines", func() {
				It("gives up after enough time", func() {
					envelope := createEnvelope(events.Envelope_LogMessage)
					conn := openTCPConnection(listener.Address(), tlsClientConfig)

					By("waiting for deadline to expire")
					time.Sleep(deadline + time.Second)

					f := func() error {
						return send(conn, envelope)
					}

					Eventually(f).ShouldNot(Succeed())
				})
			})

			Describe("shedding messages", func() {
				It("emits metric", func() {
					envelope := createEnvelope(events.Envelope_LogMessage)
					conn := openTCPConnection(listener.Address(), tlsClientConfig)

					err := send(conn, envelope)
					Expect(err).ToNot(HaveOccurred())

					Eventually(mockBatcher.BatchCounterInput, 3).Should(BeCalled(With("listeners.shedEnvelopes")))
					Eventually(mockChainer.SetTagInput, 3).Should(BeCalled(
						With("protocol", "tls"),
						With("event_type", "LogMessage"),
					))
				})

				It("downstream does not block consumption", func() {
					envelope1 := createEnvelope(events.Envelope_LogMessage)
					envelope2 := createEnvelope(events.Envelope_HttpStartStop)
					conn := openTCPConnection(listener.Address(), tlsClientConfig)

					err := send(conn, envelope1)
					Expect(err).ToNot(HaveOccurred())

					By("waiting for the first envelope to expire")
					Eventually(mockBatcher.BatchCounterInput, 3).Should(BeCalled(With("listeners.shedEnvelopes")))
					err = send(conn, envelope2)
					Expect(err).ToNot(HaveOccurred())

					var rxEnv *events.Envelope
					Eventually(envelopeChan, 3).Should(Receive(&rxEnv))
					Expect(rxEnv.GetEventType()).To(Equal(events.Envelope_HttpStartStop))
				})
			})
		})

		Describe("Start() & Stop()", func() {
			It("panics if you start again", func() {
				conn := openTCPConnection(listener.Address(), tlsClientConfig)
				defer conn.Close()

				Expect(listener.Start).To(Panic())
			})

			It("panics if you start after a stop", func() {
				conn := openTCPConnection(listener.Address(), tlsClientConfig)
				defer conn.Close()

				listener.Stop()
				Expect(listener.Start).Should(Panic())
			})

			It("fails to send message after listener has been stopped", func() {
				logMessage := factories.NewLogMessage(events.LogMessage_OUT, "some message", "appId", "source")
				envelope, _ := emitter.Wrap(logMessage, "origin")
				conn := openTCPConnection(listener.Address(), tlsClientConfig)

				err := send(conn, envelope)
				Expect(err).ToNot(HaveOccurred())

				listener.Stop()

				Eventually(func() error {
					return send(conn, envelope)
				}).Should(HaveOccurred())

				conn.Close()
			})
		})
	})
})

func readMessages(envelopeChan chan *events.Envelope, n int) []*events.Envelope {
	var envelopes []*events.Envelope
	for i := 0; i < n; i++ {
		var envelope *events.Envelope
		Eventually(envelopeChan).Should(Receive(&envelope))
		envelopes = append(envelopes, envelope)
	}
	return envelopes
}

func openTCPConnection(address string, tlsConfig *tls.Config) net.Conn {
	var (
		conn net.Conn
		err  error
	)
	Eventually(func() error {
		if tlsConfig == nil {
			conn, err = net.Dial("tcp", address)
			return err
		}
		conn, err = tls.Dial("tcp", address, tlsConfig)
		return err

	}).ShouldNot(HaveOccurred())

	return conn
}

func send(conn net.Conn, envelope *events.Envelope) error {
	bytes, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}

	var n uint32
	n = uint32(len(bytes))
	err = binary.Write(conn, binary.LittleEndian, n)
	if err != nil {
		return err
	}

	_, err = conn.Write(bytes)
	return err
}

func createEnvelope(eventType events.Envelope_EventType) *events.Envelope {
	envelope := &events.Envelope{Origin: proto.String("origin"), EventType: &eventType, Timestamp: proto.Int64(time.Now().UnixNano())}

	switch eventType {
	case events.Envelope_HttpStartStop:
		req, _ := http.NewRequest("GET", "http://www.example.com", nil)
		req.RemoteAddr = "www.example.com"
		req.Header.Add("User-Agent", "user-agent")
		uuid, _ := uuid.NewV4()
		envelope.HttpStartStop = factories.NewHttpStartStop(req, http.StatusOK, 128, events.PeerType_Client, uuid)
	case events.Envelope_ValueMetric:
		envelope.ValueMetric = factories.NewValueMetric("some-value-metric", 123, "km")
	case events.Envelope_CounterEvent:
		envelope.CounterEvent = factories.NewCounterEvent("some-counter-event", 123)
	case events.Envelope_LogMessage:
		envelope.LogMessage = factories.NewLogMessage(events.LogMessage_OUT, "some message", "appId", "source")
	case events.Envelope_ContainerMetric:
		envelope.ContainerMetric = factories.NewContainerMetric("appID", 123, 1, 5, 5)
	case events.Envelope_Error:
		envelope.Error = factories.NewError("source", 123, "message")
	default:
		panic(fmt.Sprintf("Unknown event %v\n", eventType))
	}

	return envelope
}
