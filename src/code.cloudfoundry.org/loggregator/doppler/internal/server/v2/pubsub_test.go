package v2_test

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v2"
	"code.cloudfoundry.org/loggregator/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PubSub", func() {
	var (
		pubsub *v2.PubSub
	)

	BeforeEach(func() {
		rand := &fakeRandGenerator{}
		pubsub = v2.NewPubSub(v2.WithRand(rand.Int63n))
	})

	It("writes each envelope to each subscription", func() {
		emptyReq := &loggregator_v2.EgressBatchRequest{}
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()

		pubsub.Subscribe(emptyReq, setter1)
		pubsub.Subscribe(emptyReq, setter2)

		pubsub.Publish(&loggregator_v2.Envelope{SourceId: "1"})
		pubsub.Publish(&loggregator_v2.Envelope{SourceId: "2"})

		Expect(setter1.envelopes).To(HaveLen(2))
		Expect(setter1.envelopes[0].SourceId).To(Equal("1"))
		Expect(setter1.envelopes[1].SourceId).To(Equal("2"))

		Expect(setter2.envelopes).To(HaveLen(2))
		Expect(setter2.envelopes[0].SourceId).To(Equal("1"))
		Expect(setter2.envelopes[1].SourceId).To(Equal("2"))
	})

	It("can unsubscribe from the subscription", func() {
		emptyReq := &loggregator_v2.EgressBatchRequest{}
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()

		_ = pubsub.Subscribe(emptyReq, setter1)
		unsubscribe := pubsub.Subscribe(emptyReq, setter2)
		unsubscribe()

		pubsub.Publish(&loggregator_v2.Envelope{SourceId: "1"})

		Expect(setter1.envelopes).To(HaveLen(1))
		Expect(setter1.envelopes[0].SourceId).To(Equal("1"))

		Expect(setter2.envelopes).To(HaveLen(0))
	})

	It("shards envelopes on shard id", func() {
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()
		setter3 := newSpyDataSetter()

		reqA := &loggregator_v2.EgressBatchRequest{
			ShardId: "shard-id-A",
		}
		reqB := &loggregator_v2.EgressBatchRequest{
			ShardId: "shard-id-B",
		}

		pubsub.Subscribe(reqA, setter1)
		pubsub.Subscribe(reqA, setter2)
		pubsub.Subscribe(reqB, setter3)

		for i := 0; i < 10; i++ {
			pubsub.Publish(&loggregator_v2.Envelope{SourceId: "1"})
		}

		Expect(setter1.envelopes).To(Or(HaveLen(0), HaveLen(10)))
		Expect(setter2.envelopes).To(Or(HaveLen(0), HaveLen(10)))
		Expect(len(setter1.envelopes)).ToNot(Equal(len(setter2.envelopes)))
		Expect(setter3.envelopes).To(HaveLen(10))
	})
})

type fakeRandGenerator struct {
}

func (s *fakeRandGenerator) Int63n(n int64) int64 {
	return 0
}

type spyDataSetter struct {
	envelopes []*loggregator_v2.Envelope
}

func newSpyDataSetter() *spyDataSetter {
	return &spyDataSetter{}
}

func (s *spyDataSetter) Set(e *loggregator_v2.Envelope) {
	s.envelopes = append(s.envelopes, e)
}
