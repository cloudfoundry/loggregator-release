package v1_test

import (
	"fmt"
	"io"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/plumbing"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("v1 doppler server", func() {
	var (
		mockRegistrar   *mockRegistrar
		mockCleanup     func()
		cleanupCalled   chan struct{}
		mockDataDumper  *mockDataDumper
		metricClient    *testhelper.SpyMetricClient
		healthRegistrar *SpyHealthRegistrar

		batchInterval time.Duration
		batchSize     = uint(100)

		listener      net.Listener
		connCloser    io.Closer
		dopplerClient plumbing.DopplerClient

		subscribeRequest *plumbing.SubscriptionRequest

		fakeEmitter *fake.FakeEventEmitter
	)

	BeforeSuite(func() {
		fakeEmitter = fake.NewFakeEventEmitter("doppler")
		sender := metric_sender.NewMetricSender(fakeEmitter)
		batcher := metricbatcher.New(sender, 200*time.Millisecond)
		metrics.Initialize(sender, batcher)
	})

	BeforeEach(func() {
		mockRegistrar = newMockRegistrar()
		cleanupCalled = make(chan struct{})
		mockCleanup = buildCleanup(cleanupCalled)
		mockRegistrar.RegisterOutput.Ret0 <- mockCleanup
		mockDataDumper = newMockDataDumper()
		metricClient = testhelper.NewMetricClient()
		healthRegistrar = newSpyHealthRegistrar()
		batchInterval = 50 * time.Millisecond
	})

	AfterEach(func() {
		connCloser.Close()
		listener.Close()
	})

	Describe("Subscribe", func() {
		BeforeEach(func() {
			dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
				mockRegistrar,
				mockDataDumper,
				batchInterval,
				fakeEmitter,
				healthRegistrar,
				metricClient,
			)
		})

		Context("when a stream is established", func() {
			It("registers subscription", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.RegisterInput).Should(
					BeCalled(With(subscribeRequest, Not(BeNil()))),
				)
			})

			Context("when the client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					fetchSetter(mockRegistrar)

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("when the client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					fetchSetter(mockRegistrar)
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})
			})
		})

		Describe("health registrar and metrics", func() {
			It("emits a metric for the number of subscriptions", func() {
				dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				expected := fake.Message{
					Origin: "doppler",
					Event: &events.ValueMetric{
						Name:  proto.String("grpcManager.subscriptions"),
						Value: proto.Float64(1),
						Unit:  proto.String("subscriptions"),
					},
				}

				Eventually(fakeEmitter.GetMessages, 2).Should(ContainElement(expected))
			})

			It("emits a metric for dropped egress messages", func() {
				// when you open a subscription connection, it immediately
				// starts reading from the diode
				subscription, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				setter := fetchSetter(mockRegistrar)

				done := make(chan struct{})
				defer close(done)
				go func() {
					for {
						setter.Set([]byte("some-data"))
						select {
						case <-done:
							return
						default:
						}
					}
				}()

				_, err = subscription.Recv()
				Expect(err).ToNot(HaveOccurred())

				f := func() uint64 {
					return metricClient.GetDelta("dropped")
				}
				Eventually(f, 5).Should(BeNumerically(">=", 1000))
			})

			It("increments and decrements the subscription count", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() float64 {
					return healthRegistrar.Get("subscriptionCount")
				}).Should(Equal(1.0))

				connCloser.Close()

				Eventually(func() float64 {
					return healthRegistrar.Get("subscriptionCount")
				}).Should(Equal(0.0))
			})
		})

		Describe("data transmission", func() {
			It("sends data from the setter to the client", func() {
				rx, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				setter := fetchSetter(mockRegistrar)
				setter.Set([]byte("some-data-0"))
				setter.Set([]byte("some-data-1"))
				setter.Set([]byte("some-data-2"))

				c := readFromReceiver(rx)
				Eventually(c).Should(BeCalled(With(
					[]byte("some-data-0"),
					[]byte("some-data-1"),
					[]byte("some-data-2"),
				)))
			})
		})
	})

	Describe("BatchSubscribe", func() {
		Context("when a stream is established", func() {
			BeforeEach(func() {
				dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
					mockRegistrar,
					mockDataDumper,
					batchInterval,
					fakeEmitter,
					healthRegistrar,
					metricClient,
				)
			})

			It("registers subscription", func() {
				_, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.RegisterInput).Should(
					BeCalled(With(subscribeRequest, Not(BeNil()))),
				)
			})

			Context("when the client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					fetchSetter(mockRegistrar)

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("when the client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					fetchSetter(mockRegistrar)
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})
			})
		})

		Describe("health registrar and metrics", func() {
			BeforeEach(func() {
				dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
					mockRegistrar,
					mockDataDumper,
					batchInterval,
					fakeEmitter,
					healthRegistrar,
					metricClient,
				)
			})

			It("emits a metric for the number of subscriptions", func() {
				dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				expected := fake.Message{
					Origin: "doppler",
					Event: &events.ValueMetric{
						Name:  proto.String("grpcManager.subscriptions"),
						Value: proto.Float64(1),
						Unit:  proto.String("subscriptions"),
					},
				}

				Eventually(fakeEmitter.GetMessages, 2).Should(ContainElement(expected))
			})

			It("emits a metric for dropped egress messages", func() {
				subscription, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				setter := fetchSetter(mockRegistrar)
				done := make(chan struct{})
				defer close(done)
				go func() {
					for {
						setter.Set([]byte("some-data"))
						select {
						case <-done:
							return
						default:
						}
					}
				}()

				_, err = subscription.Recv()
				Expect(err).ToNot(HaveOccurred())

				f := func() uint64 {
					return metricClient.GetDelta("dropped")
				}
				Eventually(f, 5).Should(BeNumerically(">=", 1000))
			})

			It("increments and decrements the subscription count", func() {
				_, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() float64 {
					return healthRegistrar.Get("subscriptionCount")
				}).Should(Equal(1.0))

				connCloser.Close()

				Eventually(func() float64 {
					return healthRegistrar.Get("subscriptionCount")
				}).Should(Equal(0.0))
			})
		})

		Describe("data transmission", func() {
			Context("when the batch interval expires", func() {
				BeforeEach(func() {
					dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
						mockRegistrar,
						mockDataDumper,
						batchInterval,
						fakeEmitter,
						healthRegistrar,
						metricClient,
					)
				})

				It("sends data from the setter client", func() {
					rx, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Expect(err).ToNot(HaveOccurred())

					setter := fetchSetter(mockRegistrar)
					setter.Set([]byte("some-data-0"))
					setter.Set([]byte("some-data-1"))
					setter.Set([]byte("some-data-2"))

					c := readBatchFromReceiver(rx)
					Eventually(c).Should(BeCalled(With([][]byte{
						[]byte("some-data-0"),
						[]byte("some-data-1"),
						[]byte("some-data-2"),
					})))
					Consistently(c, 500*time.Millisecond).ShouldNot(Receive())
				})
			})

			Context("when the batch size is exceeded", func() {
				BeforeEach(func() {
					dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
						mockRegistrar,
						mockDataDumper,
						time.Hour,
						fakeEmitter,
						healthRegistrar,
						metricClient,
					)
				})

				It("sends data from the setter client", func() {
					rx, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Expect(err).ToNot(HaveOccurred())

					setter := fetchSetter(mockRegistrar)
					for i := uint(0); i < (2*batchSize - 1); i++ {
						setter.Set([]byte(fmt.Sprintf("some-data-%d", i)))
					}

					c := readBatchFromReceiver(rx)

					var data [][]byte
					Eventually(c).Should(Receive(&data))
					Expect(data).To(HaveLen(int(batchSize)))
					Consistently(c, 200*time.Millisecond).ShouldNot(Receive())
				})
			})
		})
	})

	Describe("container metrics", func() {
		BeforeEach(func() {
			dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
				mockRegistrar,
				mockDataDumper,
				batchInterval,
				fakeEmitter,
				healthRegistrar,
				metricClient,
			)
		})

		It("returns container metrics from its data dumper", func() {
			envelope, data := buildContainerMetric()
			mockDataDumper.LatestContainerMetricsOutput.Ret0 <- []*events.Envelope{
				envelope,
			}

			resp, err := dopplerClient.ContainerMetrics(context.TODO(),
				&plumbing.ContainerMetricsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(ContainElement(
				data,
			))
			Expect(mockDataDumper.LatestContainerMetricsInput).To(BeCalled(
				With("some-app"),
			))
		})

		It("throw away invalid envelopes from its data dumper", func() {
			envelope, _ := buildContainerMetric()
			mockDataDumper.LatestContainerMetricsOutput.Ret0 <- []*events.Envelope{
				{},
				envelope,
			}

			resp, err := dopplerClient.ContainerMetrics(context.TODO(),
				&plumbing.ContainerMetricsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(HaveLen(1))
		})
	})

	Describe("recent logs", func() {
		BeforeEach(func() {
			dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
				mockRegistrar,
				mockDataDumper,
				batchInterval,
				fakeEmitter,
				healthRegistrar,
				metricClient,
			)
		})

		It("returns recent logs from its data dumper", func() {
			envelope, data := buildLogMessage()
			mockDataDumper.RecentLogsForOutput.Ret0 <- []*events.Envelope{
				envelope,
			}
			resp, err := dopplerClient.RecentLogs(context.TODO(),
				&plumbing.RecentLogsRequest{AppID: "some-app"})
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(ContainElement(
				data,
			))
			Expect(mockDataDumper.RecentLogsForInput).To(BeCalled(
				With("some-app"),
			))
		})

		It("throw away invalid envelopes from its data dumper", func() {
			envelope, _ := buildLogMessage()
			mockDataDumper.RecentLogsForOutput.Ret0 <- []*events.Envelope{
				{},
				envelope,
			}

			resp, err := dopplerClient.RecentLogs(context.TODO(),
				&plumbing.RecentLogsRequest{AppID: "some-app"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(HaveLen(1))
		})
	})
})

func dopplerSetup(
	mockRegistrar *mockRegistrar,
	mockDataDumper *mockDataDumper,
	batchInterval time.Duration,
	fakeEmitter *fake.FakeEventEmitter,
	healthRegistrar *SpyHealthRegistrar,
	metricClient *testhelper.SpyMetricClient,
) (
	plumbing.DopplerClient,
	*plumbing.SubscriptionRequest,
	net.Listener,
	io.Closer,
) {
	manager := v1.NewDopplerServer(
		mockRegistrar,
		mockDataDumper,
		metricClient,
		healthRegistrar,
		batchInterval,
		100,
	)

	listener := startGRPCServer(manager)
	dopplerClient, connCloser := establishClient(listener.Addr().String())

	subscribeRequest := &plumbing.SubscriptionRequest{
		Filter: &plumbing.Filter{},
	}

	fakeEmitter.Reset()
	return dopplerClient, subscribeRequest, listener, connCloser
}

func buildContainerMetric() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("some-app"),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func buildLogMessage() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_LogMessage.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte("some-log-message"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func startGRPCServer(ds plumbing.DopplerServer) net.Listener {
	lis, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	s := grpc.NewServer()
	plumbing.RegisterDopplerServer(s, ds)
	go s.Serve(lis)

	return lis
}

func establishClient(dopplerAddr string) (plumbing.DopplerClient, io.Closer) {
	conn, err := grpc.Dial(dopplerAddr, grpc.WithInsecure())
	Expect(err).ToNot(HaveOccurred())
	c := plumbing.NewDopplerClient(conn)

	return c, conn
}

func fetchSetter(mockRegistrar *mockRegistrar) v1.DataSetter {
	var s v1.DataSetter
	Eventually(mockRegistrar.RegisterInput.Setter).Should(
		Receive(&s),
	)
	return s
}

func buildCleanup(c chan struct{}) func() {
	return func() {
		close(c)
	}
}

func readFromReceiver(r plumbing.Doppler_SubscribeClient) <-chan []byte {
	c := make(chan []byte, 100)

	go func() {
		for {
			resp, err := r.Recv()
			if err != nil {
				return
			}

			c <- resp.Payload
		}
	}()

	return c
}

func readBatchFromReceiver(r plumbing.Doppler_BatchSubscribeClient) <-chan [][]byte {
	c := make(chan [][]byte, 100)

	go func() {
		for {
			resp, err := r.Recv()
			if err != nil {
				return
			}

			c <- resp.Payload
		}
	}()

	return c
}
