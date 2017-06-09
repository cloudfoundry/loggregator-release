package v1_test

import (
	"io"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("GRPCManager", func() {
	var (
		mockRegistrar   *mockRegistrar
		mockCleanup     func()
		cleanupCalled   chan struct{}
		mockDataDumper  *mockDataDumper
		metricClient    *testhelper.SpyMetricClient
		healthRegistrar *SpyHealthRegistrar

		manager       *v1.DopplerServer
		listener      net.Listener
		connCloser    io.Closer
		dopplerClient plumbing.DopplerClient

		subscribeRequest *plumbing.SubscriptionRequest

		setter      v1.DataSetter
		fakeEmitter *fake.FakeEventEmitter
	)

	var startGRPCServer = func(ds plumbing.DopplerServer) net.Listener {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		s := grpc.NewServer()
		plumbing.RegisterDopplerServer(s, ds)
		go s.Serve(lis)

		return lis
	}

	var establishClient = func(dopplerAddr string) (plumbing.DopplerClient, io.Closer) {
		conn, err := grpc.Dial(dopplerAddr, grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		c := plumbing.NewDopplerClient(conn)

		return c, conn
	}

	var fetchSetter = func() v1.DataSetter {
		var s v1.DataSetter
		Eventually(mockRegistrar.RegisterInput.Setter).Should(
			Receive(&s),
		)
		return s
	}

	var buildCleanup = func(c chan struct{}) func() {
		return func() {
			close(c)
		}
	}

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

		manager = v1.NewDopplerServer(
			mockRegistrar,
			mockDataDumper,
			metricClient,
			healthRegistrar,
		)

		listener = startGRPCServer(manager)
		dopplerClient, connCloser = establishClient(listener.Addr().String())

		subscribeRequest = &plumbing.SubscriptionRequest{
			Filter: &plumbing.Filter{},
		}

		fakeEmitter.Reset()
	})

	AfterEach(func() {
		connCloser.Close()
		listener.Close()
	})

	Describe("registration", func() {
		It("registers subscription", func() {
			_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
			Expect(err).ToNot(HaveOccurred())

			Eventually(mockRegistrar.RegisterInput).Should(
				BeCalled(With(subscribeRequest, Not(BeNil()))),
			)
		})

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
			subscription, _ := dopplerClient.Subscribe(context.TODO(), subscribeRequest)

			setter = fetchSetter()
			for i := 0; i < 2001; i++ {
				setter.Set([]byte("some-data"))
			}

			subscription.Recv()
			f := func() uint64 {
				return metricClient.GetDelta("dropped")
			}
			Eventually(f).Should(BeNumerically(">=", 1000))
		})

		Context("connection is established", func() {
			Context("client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					fetchSetter()

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					setter = fetchSetter()
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})
			})
		})

		Describe("health monitoring", func() {
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
	})

	Describe("data transmission", func() {
		var readFromReceiver = func(r plumbing.Doppler_SubscribeClient) <-chan []byte {
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

		It("sends data from the setter to the client", func() {
			rx, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
			Expect(err).ToNot(HaveOccurred())

			setter = fetchSetter()
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

	Describe("container metrics", func() {
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
