package v1_test

import (
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/doppler/internal/server/v1"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var (
		counterEnvelope      *events.Envelope
		logEnvelope          *events.Envelope
		counterEnvelopeBytes []byte
		logEnvelopeBytes     []byte

		router *v1.Router
	)

	BeforeEach(func() {
		counterEnvelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_CounterEvent.Enum(),
		}

		logEnvelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_LogMessage.Enum(),
		}
		var err error
		counterEnvelopeBytes, err = counterEnvelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		logEnvelopeBytes, err = logEnvelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		router = v1.NewRouter()
	})

	Context("with firehose subscriptions", func() {
		var (
			// Firehoses
			multipleFirehoseOfSameSubscription []*spyDataSetter
			singleFirehoseSubscription         *spyDataSetter

			requestForMultipleSubscriptions *plumbing.SubscriptionRequest
			requestForSingleSubscription    *plumbing.SubscriptionRequest

			cleanupSingleFirehose func()
		)

		BeforeEach(func() {
			multipleFirehoseOfSameSubscription = []*spyDataSetter{
				newSpyDataSetter(),
				newSpyDataSetter(),
			}
			singleFirehoseSubscription = newSpyDataSetter()

			requestForMultipleSubscriptions = &plumbing.SubscriptionRequest{
				ShardID: "some-sub-id",
			}

			requestForSingleSubscription = &plumbing.SubscriptionRequest{
				ShardID: "some-other-sub-id",
			}
			// Firehose
			router.Register(requestForMultipleSubscriptions, multipleFirehoseOfSameSubscription[0])
			router.Register(requestForMultipleSubscriptions, multipleFirehoseOfSameSubscription[1])
			cleanupSingleFirehose = router.Register(requestForSingleSubscription, singleFirehoseSubscription)
		})

		It("receives all messages", func() {
			router.SendTo("some-app-id", logEnvelope)

			Expect(singleFirehoseSubscription.setInput).To(Equal(logEnvelopeBytes))
		})

		Context("when there are two subscriptions with the same ID", func() {
			It("sends the message to one subscription", func() {
				router.SendTo("some-app-id", logEnvelope)
				combinedLen := multipleFirehoseOfSameSubscription[0].setCalls +
					multipleFirehoseOfSameSubscription[1].setCalls

				Expect(combinedLen).To(Equal(1))
			})
		})

		It("does not send messages to unregistered firehose subscriptions", func() {
			cleanupSingleFirehose()
			router.SendTo("some-app-id", counterEnvelope)

			Expect(singleFirehoseSubscription.setCalled).To(Equal(false))
		})

		It("ignores invalid envelopes", func() {
			router.SendTo("some-app-id", new(events.Envelope))

			Expect(singleFirehoseSubscription.setCalled).To(Equal(false))
		})
	})

	Context("with app ID subscriptions", func() {
		var (
			streamsForAppA             []*spyDataSetter
			streamForAppB              *spyDataSetter
			subscriptionRequestForAppB *plumbing.SubscriptionRequest

			cleanupForAppB func()
		)

		BeforeEach(func() {
			streamsForAppA = []*spyDataSetter{
				newSpyDataSetter(),
				newSpyDataSetter(),
			}

			streamForAppB = newSpyDataSetter()

			subscriptionRequestForAppA := &plumbing.SubscriptionRequest{
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
			router.Register(subscriptionRequestForAppA, streamsForAppA[0])
			router.Register(subscriptionRequestForAppA, streamsForAppA[1])
			cleanupForAppB = router.Register(subscriptionRequestForAppB, streamForAppB)
		})

		It("survives the race detector for thread safety", func(done Done) {
			cleanup := router.Register(subscriptionRequestForAppB, streamForAppB)

			go func() {
				defer close(done)
				router.SendTo("some-other-app-id", counterEnvelope)
			}()
			cleanup()
		})

		It("sends a message to all subscriptions of the same app id", func() {
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamsForAppA[0].setInput).To(Equal(counterEnvelopeBytes))
			Expect(streamsForAppA[1].setInput).To(Equal(counterEnvelopeBytes))
		})

		It("sends the envelope to a subscription only once", func() {
			router.SendTo("some-other-app-id", counterEnvelope)

			Expect(streamForAppB.setCalls).To(Equal(1))
		})

		It("does not send data to that setter", func() {
			cleanupForAppB()
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamForAppB.setCalled).To(Equal(false))
		})

		It("does not send the message to a subscription of a different app id", func() {
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamForAppB.setCalled).To(Equal(false))
		})
	})

	Context("with log filter subscriptions", func() {
		var (
			streamWithLogFilter *spyDataSetter
			subscriptionRequest *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			streamWithLogFilter = newSpyDataSetter()

			subscriptionRequest = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "some-app-id",
					Message: &plumbing.Filter_Log{
						Log: &plumbing.LogFilter{},
					},
				},
			}

			router.Register(subscriptionRequest, streamWithLogFilter)
		})

		It("sends only log messages", func() {
			router.SendTo("some-app-id", counterEnvelope)

			Expect(streamWithLogFilter.setCalled).To(Equal(false))

			router.SendTo("some-app-id", logEnvelope)

			Expect(streamWithLogFilter.setInput).To(Equal(logEnvelopeBytes))
		})
	})

	Context("with metric filter subscriptions", func() {
		var (
			stream              *spyDataSetter
			subscriptionRequest *plumbing.SubscriptionRequest
		)

		BeforeEach(func() {
			stream = newSpyDataSetter()

			subscriptionRequest = &plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					Message: &plumbing.Filter_Metric{
						Metric: &plumbing.MetricFilter{},
					},
				},
			}

			router.Register(subscriptionRequest, stream)
		})

		It("sends only metric messages", func() {
			router.SendTo("some-app-id", logEnvelope)

			Expect(stream.setCalled).To(Equal(false))

			router.SendTo("some-app-id", counterEnvelope)

			Expect(stream.setInput).To(Equal(counterEnvelopeBytes))
		})
	})
})

func newSpyDataSetter() *spyDataSetter {
	return &spyDataSetter{}
}

type spyDataSetter struct {
	setCalled bool
	setCalls  int
	setInput  []byte
}

func (s *spyDataSetter) Set(data []byte) {
	s.setCalled = true
	s.setCalls++
	s.setInput = data
}
