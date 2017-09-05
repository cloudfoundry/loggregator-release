package doppler_test

import (
	"context"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("Firehose test", func() {
	var inputConnection net.Conn
	var appID string

	BeforeEach(func() {
		guid, _ := uuid.NewV4()
		appID = guid.String()

		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
		time.Sleep(50 * time.Millisecond) // give time for connection to establish
	})

	AfterEach(func() {
		inputConnection.Close()
	})

	Context("a single firehose gets all types of logs", func() {
		var ws *websocket.Conn
		var receiveChan chan []byte

		JustBeforeEach(func() {
			receiveChan = make(chan []byte, 10)
			ws, _ = AddWSSink(receiveChan, "4567", "/firehose/hose-subcription-a")
		})

		AfterEach(func() {
			ws.Close()
		})

		It("receives log messages", func() {
			done := make(chan struct{})
			go func() {
				for {
					SendAppLog(appID, "message", inputConnection)
					select {
					case <-time.After(500 * time.Millisecond):
					case <-done:
						return
					}
				}
			}()

			var receivedMessageBytes []byte
			Eventually(receiveChan).Should(Receive(&receivedMessageBytes))
			close(done)

			Expect(DecodeProtoBufEnvelope(receivedMessageBytes).GetEventType()).To(Equal(events.Envelope_LogMessage))
		})

		It("receives container metrics", func() {
			done := make(chan struct{})
			containerMetric := factories.NewContainerMetric(appID, 0, 10, 2, 3)
			go func() {
				for {
					SendEvent(containerMetric, inputConnection)
					select {
					case <-time.After(500 * time.Millisecond):
					case <-done:
						return
					}
				}
			}()

			var receivedMessageBytes []byte
			Eventually(receiveChan).Should(Receive(&receivedMessageBytes))
			close(done)
			Expect(DecodeProtoBufEnvelope(receivedMessageBytes).GetEventType()).To(Equal(events.Envelope_ContainerMetric))
		})
	})

	Context("gRPC ingress/egress", func() {
		var (
			appID                   string
			ingressConn, egressConn *grpc.ClientConn
			ingressClient           v2.DopplerIngress_SenderClient
			egressClient            plumbing.DopplerClient
		)

		JustBeforeEach(func() {
			ingressConn, ingressClient = dopplerIngressV2Client("localhost:5678")
			guid, _ := uuid.NewV4()
			appID = guid.String()

			conf := fetchDopplerConfig(pathToConfigFile)
			egressConn, egressClient = connectToGRPC(conf)
		})

		AfterEach(func() {
			ingressConn.Close()
			egressConn.Close()
		})

		It("does not receive duplicate logs for missing app ID", func() {
			subscription, err := egressClient.Subscribe(
				context.Background(),
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
				})
			Expect(err).ToNot(HaveOccurred())

			ingressClient.Send(&v2.Envelope{
				Timestamp: time.Now().UnixNano(),
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("hello world"),
					},
				},
			})

			envs := make(chan *events.Envelope, 100)
			stop := make(chan struct{})
			go func() {
				for {
					defer GinkgoRecover()
					msg, err := subscription.Recv()
					if err != nil {
						return
					}

					e := UnmarshalMessage(msg.Payload)

					select {
					case <-stop:
						return
					case envs <- &e:
						// Do Nothing
					}
				}
			}()

			Eventually(envs, 2).Should(HaveLen(1))
			Consistently(envs).Should(HaveLen(1))
			close(stop)
		})

		It("does not receive duplicate logs for missing app ID with a filter", func() {
			subscription, err := egressClient.Subscribe(
				context.Background(),
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
					Filter: &plumbing.Filter{
						Message: &plumbing.Filter_Log{
							Log: &plumbing.LogFilter{},
						},
					},
				})
			Expect(err).ToNot(HaveOccurred())

			ingressClient.Send(&v2.Envelope{
				Timestamp: time.Now().UnixNano(),
				Message: &v2.Envelope_Log{
					Log: &v2.Log{
						Payload: []byte("hello world"),
					},
				},
			})

			envs := make(chan *events.Envelope, 100)
			stop := make(chan struct{})
			go func() {
				for {
					defer GinkgoRecover()
					msg, err := subscription.Recv()
					if err != nil {
						return
					}

					e := UnmarshalMessage(msg.Payload)

					select {
					case <-stop:
						return
					case envs <- &e:
						// Do Nothing
					}
				}
			}()

			Eventually(envs, 2).Should(HaveLen(1))
			Consistently(envs).Should(HaveLen(1))
			close(stop)
		})
	})

	It("two separate firehose subscriptions receive the same message", func() {
		receiveChan1 := make(chan []byte, 10)
		receiveChan2 := make(chan []byte, 10)
		firehoseWs1, _ := AddWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
		firehoseWs2, _ := AddWSSink(receiveChan2, "4567", "/firehose/hose-subscription-2")
		defer firehoseWs1.Close()
		defer firehoseWs2.Close()

		SendAppLog(appID, "message", inputConnection)

		receivedMessageBytes1 := []byte{}
		Eventually(receiveChan1).Should(Receive(&receivedMessageBytes1))

		receivedMessageBytes2 := []byte{}
		Eventually(receiveChan2).Should(Receive(&receivedMessageBytes2))

		receivedMessage1 := DecodeProtoBufLogMessage(receivedMessageBytes1)
		Expect(string(receivedMessage1.GetMessage())).To(Equal("message"))

		receivedMessage2 := DecodeProtoBufLogMessage(receivedMessageBytes2)
		Expect(string(receivedMessage2.GetMessage())).To(Equal("message"))
	})

	It("firehose subscriptions split message load", func() {
		receiveChan1 := make(chan []byte, 100)
		receiveChan2 := make(chan []byte, 100)
		firehoseWs1, _ := AddWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
		firehoseWs2, _ := AddWSSink(receiveChan2, "4567", "/firehose/hose-subscription-1")
		defer firehoseWs1.Close()
		defer firehoseWs2.Close()

		for i := 0; i < 100; i++ {
			SendAppLog(appID, "message", inputConnection)
		}

		Eventually(func() int {
			return len(receiveChan1) + len(receiveChan2)
		}).Should(Equal(100))

		Expect(len(receiveChan1) - len(receiveChan2)).To(BeNumerically("~", 0, 25))
	})
})
