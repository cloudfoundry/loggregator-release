package grpcmanager_test

import (
	"doppler/grpcmanager"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
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
				cleanupA, cleanupB, cleanupC, cleanupD, cleanupE, cleanupF func()
			)

			BeforeEach(func() {
				// Streams
				cleanupA = router.Register("some-app-id", false, mockDataSetterA)
				cleanupB = router.Register("some-app-id", false, mockDataSetterB)
				cleanupC = router.Register("some-other-app-id", false, mockDataSetterC)

				// Firehose
				cleanupD = router.Register("some-sub-id", true, mockDataSetterD)
				cleanupE = router.Register("some-sub-id", true, mockDataSetterE)
				cleanupF = router.Register("some-other-sub-id", true, mockDataSetterF)
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
					cleanupA = router.Register("some-app-id", false, mockDataSetterA)

					go func() {
						defer close(done)
						router.SendTo("some-app-id", envelope)
					}()
					cleanupA()
				})
			})
		})
	})
})
