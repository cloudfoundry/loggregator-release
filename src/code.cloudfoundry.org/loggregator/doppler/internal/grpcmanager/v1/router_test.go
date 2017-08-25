package v1_test

import (
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"

	. "github.com/apoydence/eachers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var (
		counterEnvelope       *events.Envelope
		logEnvelope1          *events.Envelope
		logEnvelope2          *events.Envelope
		httpStartStop1        *events.Envelope
		httpStartStop2        *events.Envelope
		containerMetric1      *events.Envelope
		containerMetric2      *events.Envelope
		counterEnvelopeBytes  []byte
		logEnvelopeBytes1     []byte
		logEnvelopeBytes2     []byte
		httpStartStopBytes1   []byte
		httpStartStopBytes2   []byte
		containerMetricBytes1 []byte
		containerMetricBytes2 []byte

		router *v1.Router
	)

	BeforeEach(func() {
		counterEnvelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_CounterEvent.Enum(),
			CounterEvent: &events.CounterEvent{
				Name:  proto.String("some-name"),
				Total: proto.Uint64(99),
				Delta: proto.Uint64(101),
			},
		}

		logEnvelope1 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Timestamp:   proto.Int64(time.Now().UnixNano()),
				Message:     []byte("some-message"),
				MessageType: events.LogMessage_OUT.Enum(),
				AppId:       proto.String("some-app-id"),
			},
		}

		logEnvelope2 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Timestamp:   proto.Int64(time.Now().UnixNano()),
				Message:     []byte("some-message"),
				MessageType: events.LogMessage_OUT.Enum(),
				AppId:       proto.String("some-other-app-id"),
			},
		}

		httpStartStop1 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_HttpStartStop.Enum(),
			HttpStartStop: &events.HttpStartStop{
				StartTimestamp: proto.Int64(time.Now().UnixNano()),
				StopTimestamp:  proto.Int64(time.Now().UnixNano()),
				RequestId:      &events.UUID{Low: proto.Uint64(0), High: proto.Uint64(0)},
				PeerType:       events.PeerType_Client.Enum(),
				Method:         events.Method_GET.Enum(),
				Uri:            proto.String("some-uri"),
				RemoteAddress:  proto.String("some-addr"),
				UserAgent:      proto.String("some-user-agent"),
				StatusCode:     proto.Int32(200),
				ContentLength:  proto.Int64(100),
				ApplicationId:  &events.UUID{Low: proto.Uint64(5), High: proto.Uint64(6)},
			},
		}

		httpStartStop2 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_HttpStartStop.Enum(),
			HttpStartStop: &events.HttpStartStop{
				StartTimestamp: proto.Int64(time.Now().UnixNano()),
				StopTimestamp:  proto.Int64(time.Now().UnixNano()),
				RequestId:      &events.UUID{Low: proto.Uint64(0), High: proto.Uint64(0)},
				PeerType:       events.PeerType_Client.Enum(),
				Method:         events.Method_GET.Enum(),
				Uri:            proto.String("some-uri"),
				RemoteAddress:  proto.String("some-addr"),
				UserAgent:      proto.String("some-user-agent"),
				StatusCode:     proto.Int32(200),
				ContentLength:  proto.Int64(100),
				ApplicationId:  &events.UUID{Low: proto.Uint64(6), High: proto.Uint64(7)},
			},
		}

		containerMetric1 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_ContainerMetric.Enum(),
			ContainerMetric: &events.ContainerMetric{
				ApplicationId: proto.String("05000000-0000-0000-0600-000000000000"),
				InstanceIndex: proto.Int32(99),
				CpuPercentage: proto.Float64(99),
				MemoryBytes:   proto.Uint64(99),
				DiskBytes:     proto.Uint64(99),
			},
		}

		containerMetric2 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_ContainerMetric.Enum(),
			ContainerMetric: &events.ContainerMetric{
				ApplicationId: proto.String("06000000-0000-0000-0700-000000000000"),
				InstanceIndex: proto.Int32(99),
				CpuPercentage: proto.Float64(99),
				MemoryBytes:   proto.Uint64(99),
				DiskBytes:     proto.Uint64(99),
			},
		}

		httpStartStop1 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_HttpStartStop.Enum(),
			HttpStartStop: &events.HttpStartStop{
				StartTimestamp: proto.Int64(time.Now().UnixNano()),
				StopTimestamp:  proto.Int64(time.Now().UnixNano()),
				RequestId:      &events.UUID{Low: proto.Uint64(0), High: proto.Uint64(0)},
				PeerType:       events.PeerType_Client.Enum(),
				Method:         events.Method_GET.Enum(),
				Uri:            proto.String("some-uri"),
				RemoteAddress:  proto.String("some-addr"),
				UserAgent:      proto.String("some-user-agent"),
				StatusCode:     proto.Int32(200),
				ContentLength:  proto.Int64(100),
				ApplicationId:  &events.UUID{Low: proto.Uint64(5), High: proto.Uint64(6)},
			},
		}

		httpStartStop2 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_HttpStartStop.Enum(),
			HttpStartStop: &events.HttpStartStop{
				StartTimestamp: proto.Int64(time.Now().UnixNano()),
				StopTimestamp:  proto.Int64(time.Now().UnixNano()),
				RequestId:      &events.UUID{Low: proto.Uint64(0), High: proto.Uint64(0)},
				PeerType:       events.PeerType_Client.Enum(),
				Method:         events.Method_GET.Enum(),
				Uri:            proto.String("some-uri"),
				RemoteAddress:  proto.String("some-addr"),
				UserAgent:      proto.String("some-user-agent"),
				StatusCode:     proto.Int32(200),
				ContentLength:  proto.Int64(100),
				ApplicationId:  &events.UUID{Low: proto.Uint64(6), High: proto.Uint64(7)},
			},
		}

		containerMetric1 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_ContainerMetric.Enum(),
			ContainerMetric: &events.ContainerMetric{
				ApplicationId: proto.String("05000000-0000-0000-0600-000000000000"),
				InstanceIndex: proto.Int32(99),
				CpuPercentage: proto.Float64(99),
				MemoryBytes:   proto.Uint64(99),
				DiskBytes:     proto.Uint64(99),
			},
		}

		containerMetric2 = &events.Envelope{
			Origin:    proto.String("some-origin"),
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_ContainerMetric.Enum(),
			ContainerMetric: &events.ContainerMetric{
				ApplicationId: proto.String("06000000-0000-0000-0700-000000000000"),
				InstanceIndex: proto.Int32(99),
				CpuPercentage: proto.Float64(99),
				MemoryBytes:   proto.Uint64(99),
				DiskBytes:     proto.Uint64(99),
			},
		}

		var err error
		counterEnvelopeBytes, err = counterEnvelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		logEnvelopeBytes1, err = logEnvelope1.Marshal()
		Expect(err).ToNot(HaveOccurred())

		logEnvelopeBytes2, err = logEnvelope2.Marshal()
		Expect(err).ToNot(HaveOccurred())

		httpStartStopBytes1, err = httpStartStop1.Marshal()
		Expect(err).ToNot(HaveOccurred())

		httpStartStopBytes2, err = httpStartStop2.Marshal()
		Expect(err).ToNot(HaveOccurred())

		containerMetricBytes1, err = containerMetric1.Marshal()
		Expect(err).ToNot(HaveOccurred())

		containerMetricBytes2, err = containerMetric2.Marshal()
		Expect(err).ToNot(HaveOccurred())

		router = v1.NewRouter()
	})

	Context("with firehose subscriptions", func() {
		var (
			// Firehoses
			multipleFirehoseOfSameSubscription []*mockDataSetter
			singleFirehoseSubscription         *mockDataSetter

			requestForMultipleSubscriptions *plumbing.SubscriptionRequest
			requestForSingleSubscription    *plumbing.SubscriptionRequest

			cleanupSingleFirehose func()
		)

		BeforeEach(func() {
			multipleFirehoseOfSameSubscription = []*mockDataSetter{
				newMockDataSetter(),
				newMockDataSetter(),
			}
			singleFirehoseSubscription = newMockDataSetter()

			requestForMultipleSubscriptions = &plumbing.SubscriptionRequest{
				ShardID: "some-sub-id",
			}

			requestForSingleSubscription = &plumbing.SubscriptionRequest{
				ShardID: "some-other-sub-id",
			}
			// Firehose
			router.Register(requestForMultipleSubscriptions, multipleFirehoseOfSameSubscription[0].Set)
			router.Register(requestForMultipleSubscriptions, multipleFirehoseOfSameSubscription[1].Set)
			cleanupSingleFirehose = router.Register(requestForSingleSubscription, singleFirehoseSubscription.Set)
		})

		It("receives all messages", func() {
			router.SendTo("some-app-id", logEnvelope1)
			router.SendTo("some-app-id", counterEnvelope)

			Expect(singleFirehoseSubscription.SetInput).To(
				BeCalled(With(logEnvelopeBytes1)),
			)
			Expect(singleFirehoseSubscription.SetInput).To(
				BeCalled(With(counterEnvelopeBytes)),
			)
		})

		Context("when there are two subscriptions with the same ID", func() {
			It("sends the message to one subscription", func() {
				router.SendTo("some-app-id", logEnvelope1)
				combinedLen := len(multipleFirehoseOfSameSubscription[0].SetCalled) + len(multipleFirehoseOfSameSubscription[1].SetCalled)

				Expect(combinedLen).To(Equal(1))
			})
		})

		It("does not send messages to unregistered firehose subscriptions", func() {
			cleanupSingleFirehose()
			router.SendTo("some-app-id", counterEnvelope)

			Expect(singleFirehoseSubscription.SetInput).To(
				Not(BeCalled()),
			)
		})

		It("ignores invalid envelopes", func() {
			router.SendTo("some-app-id", new(events.Envelope))

			Expect(singleFirehoseSubscription.SetCalled).To(
				Not(BeCalled()),
			)
		})
	})

	Context("with app ID subscriptions", func() {
		var (
			streamsForAppA             []*mockDataSetter
			streamForAppB              *mockDataSetter
			subscriptionRequestForAppA *plumbing.SubscriptionRequest
			subscriptionRequestForAppB *plumbing.SubscriptionRequest

			cleanupForAppA []func()
			cleanupForAppB func()
		)

		BeforeEach(func() {
			streamsForAppA = []*mockDataSetter{
				newMockDataSetter(),
				newMockDataSetter(),
			}

			streamForAppB = newMockDataSetter()

			subscriptionRequestForAppA = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "some-app-id",
				},
			}

			subscriptionRequestForAppB = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "some-other-app-id",
				},
			}
			// Streams without type filters
			cleanupForAppA = []func(){
				router.Register(subscriptionRequestForAppA, streamsForAppA[0].Set),
				router.Register(subscriptionRequestForAppA, streamsForAppA[1].Set),
			}
			cleanupForAppB = router.Register(subscriptionRequestForAppB, streamForAppB.Set)
		})

		It("survives the race detector for thread safety", func(done Done) {
			cleanup := router.Register(subscriptionRequestForAppB, streamForAppB.Set)

			go func() {
				defer close(done)
				router.SendTo("some-other-app-id", counterEnvelope)
			}()
			cleanup()
		})

		It("sends a message to all subscriptions of the same app id", func() {
			router.SendTo("some-app-id", logEnvelope1)

			Expect(streamsForAppA[0].SetInput).To(
				BeCalled(With(logEnvelopeBytes1)),
			)
			Expect(streamsForAppA[1].SetInput).To(
				BeCalled(With(logEnvelopeBytes1)),
			)
		})

		It("sends the envelope to a subscription only once", func() {
			router.SendTo("some-other-app-id", logEnvelope2)

			Expect(streamForAppB.SetCalled).To(
				HaveLen(1),
			)
		})

		It("does not send data to that setter", func() {
			cleanupForAppB()
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamForAppB.SetCalled).To(
				Not(BeCalled()),
			)
		})

		It("does not send the message to a subscription of a different app id", func() {
			router.SendTo("some-app-id", logEnvelope1)

			Expect(streamForAppB.SetInput).To(
				Not(BeCalled()),
			)
		})
	})

	Context("with log filter subscriptions", func() {
		var (
			streamWithLogFilter *mockDataSetter
			subscriptionRequest *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			streamWithLogFilter = newMockDataSetter()

			subscriptionRequest = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "some-app-id",
					Message: &plumbing.Filter_Log{
						Log: &plumbing.LogFilter{},
					},
				},
			}

			router.Register(subscriptionRequest, streamWithLogFilter.Set)
		})

		It("sends only log messages", func() {
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamWithLogFilter.SetCalled).To(
				Not(BeCalled()),
			)

			router.SendTo("some-app-id", logEnvelope1)

			Expect(streamWithLogFilter.SetInput).To(
				BeCalled(With(logEnvelopeBytes1)),
			)
		})
	})

	Context("with metric filter subscriptions", func() {
		var (
			stream              *mockDataSetter
			subscriptionRequest *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			stream = newMockDataSetter()

			subscriptionRequest = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					Message: &plumbing.Filter_Metric{
						Metric: &plumbing.MetricFilter{},
					},
				},
			}

			router.Register(subscriptionRequest, stream.Set)
		})

		It("sends only metric messages", func() {
			router.SendTo("some-app-id", logEnvelope1)

			Expect(stream.SetCalled).To(
				Not(BeCalled()),
			)

			router.SendTo("some-app-id", counterEnvelope)

			Expect(stream.SetInput).To(
				BeCalled(With(counterEnvelopeBytes)),
			)
		})
	})

	Context("with metric and appID filter subscriptions", func() {
		var (
			stream              *mockDataSetter
			subscriptionRequest *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			stream = newMockDataSetter()

			subscriptionRequest = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "05000000-0000-0000-0600-000000000000",
					Message: &plumbing.Filter_Metric{
						Metric: &plumbing.MetricFilter{},
					},
				},
			}

			router.Register(subscriptionRequest, stream.Set)
		})

		It("sends only metric messages with the right appID", func() {
			router.SendTo("05000000-0000-0000-0600-000000000000", logEnvelope1)

			Expect(stream.SetCalled).To(
				Not(BeCalled()),
			)

			// CounterEvents do not have an appID, so they should not be sent
			router.SendTo("05000000-0000-0000-0600-000000000000", counterEnvelope)

			Expect(stream.SetInput).ToNot(
				BeCalled(With(counterEnvelopeBytes)),
			)

			router.SendTo("05000000-0000-0000-0600-000000000000", httpStartStop1)

			Expect(stream.SetInput).To(
				BeCalled(With(httpStartStopBytes1)),
			)

			router.SendTo("06000000-0000-0000-0700-000000000000", httpStartStop2)

			Expect(stream.SetInput).ToNot(
				BeCalled(With(httpStartStopBytes1)),
			)

			router.SendTo("05000000-0000-0000-0600-000000000000", containerMetric1)

			Expect(stream.SetInput).To(
				BeCalled(With(containerMetricBytes1)),
			)

			router.SendTo("06000000-0000-0000-0700-000000000000", containerMetric2)

			Expect(stream.SetInput).ToNot(
				BeCalled(With(containerMetric2)),
			)
		})
	})
})
