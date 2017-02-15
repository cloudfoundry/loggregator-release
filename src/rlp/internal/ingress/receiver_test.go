package ingress_test

import (
	"context"
	"fmt"
	"plumbing"
	v2 "plumbing/v2"
	"rlp/internal/ingress"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Receiver", func() {
	var (
		mockConverter  *mockConverter
		mockSubscriber *mockSubscriber
		rx             *ingress.Receiver
	)

	BeforeEach(func() {
		mockConverter = newMockConverter()
		mockSubscriber = newMockSubscriber()
		rx = ingress.NewReceiver(mockConverter, mockSubscriber)
	})

	Context("when the subscriber does not return an error", func() {
		BeforeEach(func() {
			mockSubscriber.SubscribeOutput.Recv <- func() ([]byte, error) {
				return []byte("something"), nil
			}
			close(mockSubscriber.SubscribeOutput.Err)
		})

		It("subscribes to data", func() {
			req := &v2.EgressRequest{
				ShardId: "some-id",
				Filter: &v2.Filter{
					SourceId: "some-source-id",
				},
			}
			expectedReq := &plumbing.SubscriptionRequest{
				ShardID: req.ShardId,
				Filter:  &plumbing.Filter{AppID: "some-source-id"},
			}
			rx.Subscribe(context.Background(), req)

			Expect(mockSubscriber.SubscribeInput.Ctx).To(Receive(Not(BeNil())))
			Expect(mockSubscriber.SubscribeInput.Req).To(Receive(Equal(expectedReq)))
		})

		It("converts the data", func() {
			close(mockConverter.ConvertOutput.Envelope)
			close(mockConverter.ConvertOutput.Err)
			req := &v2.EgressRequest{}
			rx, err := rx.Subscribe(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())
			rx()

			Expect(mockConverter.ConvertInput.Data).To(Receive(Equal(
				[]byte("something"),
			)))
		})

		It("returns an error if the convert fails", func() {
			close(mockConverter.ConvertOutput.Envelope)
			mockConverter.ConvertOutput.Err <- fmt.Errorf("some-error")
			req := &v2.EgressRequest{}
			rx, err := rx.Subscribe(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())
			_, err = rx()

			Expect(err).To(HaveOccurred())
		})

		It("streams the converted data", func() {
			expectedEnv := &v2.Envelope{
				Timestamp: 1,
			}
			mockConverter.ConvertOutput.Envelope <- expectedEnv
			close(mockConverter.ConvertOutput.Err)

			req := &v2.EgressRequest{}
			rx, err := rx.Subscribe(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())

			env, err := rx()
			Expect(err).ToNot(HaveOccurred())
			Expect(env).To(Equal(expectedEnv))
		})
	})

	Context("when the subscriber receiver errors", func() {
		BeforeEach(func() {
			mockSubscriber.SubscribeOutput.Recv <- func() ([]byte, error) {
				return nil, fmt.Errorf("some-error")
			}
			close(mockSubscriber.SubscribeOutput.Err)
			close(mockConverter.ConvertOutput.Err)
			close(mockConverter.ConvertOutput.Envelope)
		})

		It("returns an error via the receiver", func() {
			req := &v2.EgressRequest{}
			rx, err := rx.Subscribe(context.Background(), req)
			Expect(err).ToNot(HaveOccurred())

			_, err = rx()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the subscriber returns an error", func() {
		BeforeEach(func() {
			close(mockSubscriber.SubscribeOutput.Recv)
			mockSubscriber.SubscribeOutput.Err <- fmt.Errorf("some-error")
		})

		It("returns an error", func() {
			req := &v2.EgressRequest{}
			_, err := rx.Subscribe(context.Background(), req)
			Expect(err).To(HaveOccurred())
		})
	})

})
