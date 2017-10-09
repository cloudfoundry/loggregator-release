package v2_test

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v2"
	"code.cloudfoundry.org/loggregator/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("PubSub", func() {
	var (
		pubsub *v2.PubSub
	)

	BeforeEach(func() {
		fakeRand := &fakeRandGenerator{}
		pubsub = v2.NewPubSub(v2.WithRand(fakeRand.Int63n))
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

	Describe("Selectors", func() {
		DescribeTable("selects only the requested types",
			func(s *loggregator_v2.Selector, t interface{}) {
				req := &loggregator_v2.EgressBatchRequest{
					Selectors: []*loggregator_v2.Selector{s},
				}
				setter := newSpyDataSetter()

				pubsub.Subscribe(req, setter)
				publishAllTypes(pubsub, "some-id")

				Expect(setter.envelopes).To(HaveLen(1))
				env := setter.envelopes[0]
				Expect(env.GetMessage()).ToNot(BeNil())
				Expect(env.GetMessage()).To(BeAssignableToTypeOf(t))
			},
			Entry("Log", &loggregator_v2.Selector{
				Message: &loggregator_v2.Selector_Log{
					Log: &loggregator_v2.LogSelector{},
				},
			}, &loggregator_v2.Envelope_Log{}),
			Entry("Counter", &loggregator_v2.Selector{
				Message: &loggregator_v2.Selector_Counter{
					Counter: &loggregator_v2.CounterSelector{},
				},
			}, &loggregator_v2.Envelope_Counter{}),
			Entry("Gauge", &loggregator_v2.Selector{
				Message: &loggregator_v2.Selector_Gauge{
					Gauge: &loggregator_v2.GaugeSelector{},
				},
			}, &loggregator_v2.Envelope_Gauge{}),
			Entry("Timer", &loggregator_v2.Selector{
				Message: &loggregator_v2.Selector_Timer{
					Timer: &loggregator_v2.TimerSelector{},
				},
			}, &loggregator_v2.Envelope_Timer{}),
			Entry("Event", &loggregator_v2.Selector{
				Message: &loggregator_v2.Selector_Event{
					Event: &loggregator_v2.EventSelector{},
				},
			}, &loggregator_v2.Envelope_Event{}),
		)

		It("selects all types with no selector", func() {
			req := &loggregator_v2.EgressBatchRequest{}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			publishAllTypes(pubsub, "some-id")

			Expect(setter.envelopes).To(HaveLen(5))
		})

		It("selects by source ID", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{SourceId: "a"},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")

			Expect(setter.envelopes).To(HaveLen(5))
		})

		It("supports multiple selectors", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: "a",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
					{
						SourceId: "a",
						Message: &loggregator_v2.Selector_Gauge{
							Gauge: &loggregator_v2.GaugeSelector{},
						},
					},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")
			publishAllTypes(pubsub, "c")

			Expect(setter.envelopes).To(HaveLen(2))
		})

		// With the current implementation we get duplicate logs in this case.
		// Selector 1 gets all envelopes for SourceId "a".
		// Selector 2 gets all log envelopes for SourceId "a", "b", "c"
		// Selector 3 gets the counter envelope for source id "c"
		XIt("supports any combination of selectors", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{SourceId: "a"}, // Selector 1
					{ // Selector 2
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
					{ // Selector 3
						SourceId: "c",
						Message: &loggregator_v2.Selector_Counter{
							Counter: &loggregator_v2.CounterSelector{},
						},
					},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")
			publishAllTypes(pubsub, "c")
			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")
			publishAllTypes(pubsub, "c")

			Expect(setter.envelopes).To(HaveLen(14))
		})

		It("handles subscriptions with same shard ID but different selectors", func() {
			req1 := &loggregator_v2.EgressBatchRequest{
				ShardId: "a-shard-id",
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}
			req2 := &loggregator_v2.EgressBatchRequest{
				ShardId: "a-shard-id",
				Selectors: []*loggregator_v2.Selector{
					{SourceId: "a"},
				},
			}

			setter1 := newSpyDataSetter()
			setter2 := newSpyDataSetter()

			pubsub.Subscribe(req1, setter1)
			pubsub.Subscribe(req2, setter2)

			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")

			Expect(setter1.envelopes).To(HaveLen(2))
			Expect(setter2.envelopes).To(HaveLen(5))
		})
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

func publishAllTypes(pubsub *v2.PubSub, id string) {
	pubsub.Publish(buildLog(id))
	pubsub.Publish(buildCounter(id))
	pubsub.Publish(buildGauge(id))
	pubsub.Publish(buildTimer(id))
	pubsub.Publish(buildEvent(id))
}

func buildLog(id string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte("Hello Loggregator Team!"),
				Type:    loggregator_v2.Log_OUT,
			},
		},
	}
}

func buildCounter(id string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  "my-time",
				Delta: 1234,
				Total: 123454,
			},
		},
	}
}

func buildGauge(id string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"my-metric": {
						Unit:  "bytes",
						Value: 12342.23,
					},
				},
			},
		},
	}
}

func buildTimer(id string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Timer{
			Timer: &loggregator_v2.Timer{
				Name:  "my-time",
				Start: 1234,
				Stop:  123454,
			},
		},
	}
}

func buildEvent(id string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Event{
			Event: &loggregator_v2.Event{
				Title: "some-title",
				Body:  "some-body",
			},
		},
	}
}
