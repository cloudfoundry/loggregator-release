package grpcmanager_test

import (
	"doppler/grpcmanager"
	"plumbing"

	. "github.com/apoydence/eachers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var (
		// Streams
		mockDataSetterA *mockDataSetter
		mockDataSetterB *mockDataSetter
		mockDataSetterC *mockDataSetter

		// Firehoses
		mockDataSetterD *mockDataSetter
		mockDataSetterE *mockDataSetter
		mockDataSetterF *mockDataSetter

		envelope      *events.Envelope
		envelopeBytes []byte

		router *grpcmanager.Router
	)

	BeforeEach(func() {
		mockDataSetterA = newMockDataSetter()
		mockDataSetterB = newMockDataSetter()
		mockDataSetterC = newMockDataSetter()
		mockDataSetterD = newMockDataSetter()
		mockDataSetterE = newMockDataSetter()
		mockDataSetterF = newMockDataSetter()

		envelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_CounterEvent.Enum(),
		}
		var err error
		envelopeBytes, err = envelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		router = grpcmanager.NewRouter()
	})

	Describe("data routing", func() {
		Context("when multiple setters are routed", func() {
			var (
				reqA, reqB, reqC, reqD, reqE, reqF                         *plumbing.SubscriptionRequest
				cleanupA, cleanupB, cleanupC, cleanupD, cleanupE, cleanupF func()
			)

			BeforeEach(func() {
				reqA = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-app-id",
					},
				}

				reqB = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-app-id",
					},
				}

				reqC = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-other-app-id",
					},
				}

				reqD = &plumbing.SubscriptionRequest{
					ShardID: "some-sub-id",
				}

				reqE = &plumbing.SubscriptionRequest{
					ShardID: "some-sub-id",
				}

				reqF = &plumbing.SubscriptionRequest{
					ShardID: "some-other-sub-id",
				}

				// Streams
				cleanupA = router.Register(reqA, mockDataSetterA)
				cleanupB = router.Register(reqB, mockDataSetterB)
				cleanupC = router.Register(reqC, mockDataSetterC)

				// Firehose
				cleanupD = router.Register(reqD, mockDataSetterD)
				cleanupE = router.Register(reqE, mockDataSetterE)
				cleanupF = router.Register(reqF, mockDataSetterF)
			})

			It("sends data to the registered setters", func() {
				router.SendTo("some-app-id", envelope)

				Eventually(mockDataSetterA.SetInput).Should(
					BeCalled(With(envelopeBytes)),
				)

				Eventually(mockDataSetterB.SetInput).Should(
					BeCalled(With(envelopeBytes)),
				)
			})

			It("does not send data to the wrong setter", func() {
				router.SendTo("some-app-id", envelope)

				Consistently(mockDataSetterC.SetCalled).Should(
					Not(BeCalled()),
				)
			})

			It("sends to a random firehose subscription", func() {
				router.SendTo("some-app-id", envelope)

				f := func() int {
					return len(mockDataSetterD.SetCalled) + len(mockDataSetterE.SetCalled)
				}

				Eventually(f).Should(Equal(1))

				Eventually(mockDataSetterF.SetInput).Should(
					BeCalled(With(envelopeBytes)),
				)
			})

			It("does not send data for bad envelope", func() {
				router.SendTo("some-app-id", new(events.Envelope))

				Consistently(mockDataSetterA.SetCalled).Should(
					Not(BeCalled()),
				)

				Consistently(mockDataSetterD.SetCalled).Should(
					Not(BeCalled()),
				)

				Consistently(mockDataSetterE.SetCalled).Should(
					Not(BeCalled()),
				)
			})

			Context("when one stream setter is unregistered", func() {
				BeforeEach(func() {
					cleanupA()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", envelope)

					Consistently(mockDataSetterA.SetCalled).Should(
						Not(BeCalled()),
					)
				})

				It("does send data to the remaining registered setter", func() {
					router.SendTo("some-app-id", envelope)

					Eventually(mockDataSetterB.SetInput).Should(
						BeCalled(With(envelopeBytes)),
					)
				})
			})

			Context("when one firehoses subscription is unregistered", func() {
				BeforeEach(func() {
					cleanupD()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", envelope)

					Consistently(mockDataSetterD.SetCalled).Should(
						Not(BeCalled()),
					)
				})
			})

			Context("when one of one firehoses subscription is unregistered", func() {
				BeforeEach(func() {
					cleanupF()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", envelope)

					Consistently(mockDataSetterF.SetCalled).Should(
						Not(BeCalled()),
					)
				})
			})

			Describe("thread safety", func() {
				It("survives the race detector", func(done Done) {
					cleanup := router.Register(reqA, mockDataSetterA)

					go func() {
						defer close(done)
						router.SendTo("some-app-id", envelope)
					}()
					cleanup()
				})
			})
		})
	})
})
