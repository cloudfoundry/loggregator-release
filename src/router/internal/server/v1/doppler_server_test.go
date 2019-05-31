package v1_test

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/router/internal/server/v1"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("v1 doppler server", func() {
	var (
		mockRegistrar   *spyRegistrar
		mockCleanup     func()
		cleanupCalled   chan struct{}
		mockDataDumper  *spyDataDumper
		healthRegistrar *SpyHealthRegistrar

		metricClient        *testhelper.SpyMetricClient
		egressDropped       *metricemitter.Counter
		subscriptionsMetric *metricemitter.Gauge

		batchInterval time.Duration
		batchSize     = uint(100)

		listener      net.Listener
		connCloser    io.Closer
		dopplerClient plumbing.DopplerClient

		subscribeRequest *plumbing.SubscriptionRequest
	)

	BeforeEach(func() {
		mockRegistrar = newSpyRegistrar()
		cleanupCalled = make(chan struct{})
		mockCleanup = buildCleanup(cleanupCalled)
		mockRegistrar.cleanup = mockCleanup
		mockDataDumper = newSpyDataDumper()
		healthRegistrar = newSpyHealthRegistrar()
		batchInterval = 50 * time.Millisecond

		metricClient = testhelper.NewMetricClient()
		egressDropped = &metricemitter.Counter{}
		subscriptionsMetric = metricClient.NewGauge("subs", "subscriptions")
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
				healthRegistrar,
				metricClient,
				egressDropped,
				subscriptionsMetric,
			)
		})

		Context("when a stream is established", func() {
			It("registers subscription", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.registerRequest).Should(Equal(subscribeRequest))
			})

			Context("when the client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("when the client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.Subscribe(context.TODO(), subscribeRequest)
					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})
			})
		})

		Describe("health registrar and metrics", func() {
			It("emits a metric for number of envelopes egressed", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())

				setter := mockRegistrar.registerSetter()
				setter.Set([]byte("some-data-0"))
				setter.Set([]byte("some-data-0"))
				setter.Set([]byte("some-data-0"))

				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(Equal(uint64(3)))
			})

			It("emits a metric for the number of subscriptions", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() float64 {
					return subscriptionsMetric.GetValue()
				}).Should(Equal(1.0))

				connCloser.Close()

				Eventually(func() float64 {
					return subscriptionsMetric.GetValue()
				}).Should(Equal(0.0))
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

			It("emits a metric for the egress dropped", func() {
				_, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())
				Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())

				setter := mockRegistrar.registerSetter()
				for i := 0; i < 1500; i++ {
					setter.Set([]byte("some-data-0"))
				}

				Eventually(egressDropped.GetDelta).Should(BeNumerically("==", 1000))
			})
		})

		Describe("data transmission", func() {
			It("sends data from the setter to the client", func() {
				rx, err := dopplerClient.Subscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
				setter := mockRegistrar.registerSetter()
				setter.Set([]byte("some-data-0"))

				c := readFromReceiver(rx)
				var data []byte
				Eventually(c).Should(Receive(&data))
				Expect(data).To(Equal([]byte("some-data-0")))
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
					healthRegistrar,
					metricClient,
					egressDropped,
					subscriptionsMetric,
				)

			})

			It("registers subscription", func() {
				_, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.registerRequest).Should(Equal(subscribeRequest))
			})

			Context("when the client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("when the client closes connection", func() {
				It("unregisters subscription", func() {
					dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
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
					healthRegistrar,
					metricClient,
					egressDropped,
					subscriptionsMetric,
				)

			})

			It("emits a metric for number of envelopes egressed", func() {
				_, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
				setter := mockRegistrar.registerSetter()
				setter.Set([]byte("some-data-0"))
				setter.Set([]byte("some-data-0"))
				setter.Set([]byte("some-data-0"))

				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(Equal(uint64(3)))
			})

			It("emits a metric for the number of subscriptions", func() {
				_, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() float64 {
					return subscriptionsMetric.GetValue()
				}).Should(Equal(1.0))

				connCloser.Close()

				Eventually(func() float64 {
					return subscriptionsMetric.GetValue()
				}).Should(Equal(0.0))
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
						healthRegistrar,
						metricClient,
						egressDropped,
						subscriptionsMetric,
					)

				})

				It("sends data from the setter client", func() {
					rx, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Expect(err).ToNot(HaveOccurred())

					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
					setter := mockRegistrar.registerSetter()
					setter.Set([]byte("some-data-0"))

					c := readBatchFromReceiver(rx)
					var data [][]byte
					Eventually(c, 2).Should(Receive(&data))
					Expect(data).To(HaveLen(int(1)))
					Expect(data[0]).To(Equal([]byte("some-data-0")))
					Consistently(c, 500*time.Millisecond).ShouldNot(Receive())
				})
			})

			Context("when the batch size is exceeded", func() {
				BeforeEach(func() {
					dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
						mockRegistrar,
						mockDataDumper,
						time.Hour,
						healthRegistrar,
						metricClient,
						egressDropped,
						subscriptionsMetric,
					)

				})

				It("sends data from the setter client", func() {
					rx, err := dopplerClient.BatchSubscribe(context.TODO(), subscribeRequest)
					Expect(err).ToNot(HaveOccurred())

					Eventually(mockRegistrar.registerSetter).ShouldNot(BeNil())
					setter := mockRegistrar.registerSetter()
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

	Describe("recent logs", func() {
		BeforeEach(func() {
			dopplerClient, subscribeRequest, listener, connCloser = dopplerSetup(
				mockRegistrar,
				mockDataDumper,
				batchInterval,
				healthRegistrar,
				metricClient,
				egressDropped,
				subscriptionsMetric,
			)

		})

		It("returns recent logs from its data dumper", func() {
			envelope, data := buildLogMessage()
			mockDataDumper.recentLogsForEnvelopes = []*events.Envelope{
				envelope,
			}
			resp, err := dopplerClient.RecentLogs(context.TODO(),
				&plumbing.RecentLogsRequest{AppID: "some-app"})
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Payload).To(ContainElement(
				data,
			))
			Expect(mockDataDumper.recentLogsForAppID).To(Equal("some-app"))
		})

		It("throw away invalid envelopes from its data dumper", func() {
			envelope, _ := buildLogMessage()
			mockDataDumper.recentLogsForEnvelopes = []*events.Envelope{
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
	mockRegistrar *spyRegistrar,
	mockDataDumper *spyDataDumper,
	batchInterval time.Duration,
	healthRegistrar *SpyHealthRegistrar,
	metricClient *testhelper.SpyMetricClient,
	droppedMetric *metricemitter.Counter,
	subscriptionsMetric *metricemitter.Gauge,
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
		droppedMetric,
		subscriptionsMetric,
		healthRegistrar,
		batchInterval,
		100,
	)

	listener := startGRPCServer(manager)
	dopplerClient, connCloser := establishClient(listener.Addr().String())

	subscribeRequest := &plumbing.SubscriptionRequest{
		Filter: &plumbing.Filter{},
	}

	return dopplerClient, subscribeRequest, listener, connCloser
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
		Tags: make(map[string]string),
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func startGRPCServer(ds plumbing.DopplerServer) net.Listener {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
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

func newSpyRegistrar() *spyRegistrar {
	return &spyRegistrar{}
}

type spyRegistrar struct {
	mu               sync.Mutex
	registerRequest_ *plumbing.SubscriptionRequest
	registerSetter_  v1.DataSetter
	cleanup          func()
}

func (s *spyRegistrar) Register(req *plumbing.SubscriptionRequest, setter v1.DataSetter) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerRequest_ = req
	s.registerSetter_ = setter
	return s.cleanup
}

func (s *spyRegistrar) registerRequest() *plumbing.SubscriptionRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.registerRequest_
}

func (s *spyRegistrar) registerSetter() v1.DataSetter {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.registerSetter_
}

func newSpyDataDumper() *spyDataDumper {
	return &spyDataDumper{}
}

type spyDataDumper struct {
	recentLogsForAppID     string
	recentLogsForEnvelopes []*events.Envelope
}

func (s *spyDataDumper) RecentLogsFor(appID string) []*events.Envelope {
	s.recentLogsForAppID = appID

	return s.recentLogsForEnvelopes
}
