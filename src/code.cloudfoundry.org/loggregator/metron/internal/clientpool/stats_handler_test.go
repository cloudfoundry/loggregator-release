package clientpool_test

import (
	"context"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
	"google.golang.org/grpc/stats"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StatsHandler", func() {
	var (
		tracker *spyTracker
		handler stats.Handler
	)

	BeforeEach(func() {
		tracker = &spyTracker{}
		handler = clientpool.NewStatsHandler(tracker)
	})

	Describe("V1", func() {
		It("tracks each envelope", func() {
			handler.HandleRPC(context.Background(), &stats.OutPayload{
				Payload: &plumbing.EnvelopeData{
					Payload: make([]byte, 99),
				},
			})

			Expect(tracker.lastSize).To(Equal(99))
		})
	})

	Describe("V2 Batches", func() {
		It("tracks each batch", func() {
			handler.HandleRPC(context.Background(), &stats.OutPayload{
				Payload: &loggregator_v2.EnvelopeBatch{
					Batch: make([]*loggregator_v2.Envelope, 3),
				},
				Length: 99,
			})

			Expect(tracker.lastCount).To(Equal(3))
			Expect(tracker.lastSize).To(Equal(99))
		})

		It("handles a batch length of 0", func() {
			handler.HandleRPC(context.Background(), &stats.OutPayload{
				Payload: &loggregator_v2.EnvelopeBatch{
					Batch: nil,
				},
				Length: 99,
			})

			Expect(tracker.lastCount).To(Equal(0))
		})
	})

	Describe("V2 Envelope", func() {
		It("tracks each envelope", func() {
			handler.HandleRPC(context.Background(), &stats.OutPayload{
				Payload: &loggregator_v2.Envelope{},
				Length:  99,
			})

			Expect(tracker.lastSize).To(Equal(99))
		})
	})

	Describe("unknown payload types", func() {
		It("ignores non OutPayload messages", func() {
			f := func() {
				handler.HandleRPC(context.Background(), &stats.InPayload{})
			}

			Expect(f).ToNot(Panic())
		})

		It("ignores unknown OutPayload.Payload types", func() {
			f := func() {
				handler.HandleRPC(context.Background(), &stats.OutPayload{
					Payload: "invalid",
				})
			}

			Expect(f).ToNot(Panic())
		})
	})
})

type spyTracker struct {
	lastCount int
	lastSize  int
}

func (s *spyTracker) Track(count, size int) {
	s.lastCount = count
	s.lastSize = size
}
