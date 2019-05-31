package v2_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/router/internal/server/v2"

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
		req := &loggregator_v2.EgressBatchRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		}
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()

		pubsub.Subscribe(req, setter1)
		pubsub.Subscribe(req, setter2)

		pubsub.Publish(&loggregator_v2.Envelope{
			SourceId: "1",
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{},
			},
		})
		pubsub.Publish(&loggregator_v2.Envelope{
			SourceId: "2",
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{},
			},
		})

		Expect(setter1.envelopes).To(HaveLen(2))
		Expect(setter1.envelopes[0].SourceId).To(Equal("1"))
		Expect(setter1.envelopes[1].SourceId).To(Equal("2"))

		Expect(setter2.envelopes).To(HaveLen(2))
		Expect(setter2.envelopes[0].SourceId).To(Equal("1"))
		Expect(setter2.envelopes[1].SourceId).To(Equal("2"))
	})

	It("can unsubscribe from the subscription", func() {
		req := &loggregator_v2.EgressBatchRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		}
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()

		_ = pubsub.Subscribe(req, setter1)
		unsubscribe := pubsub.Subscribe(req, setter2)
		unsubscribe()

		pubsub.Publish(&loggregator_v2.Envelope{
			SourceId: "1",
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{},
			},
		})

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
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		}
		reqB := &loggregator_v2.EgressBatchRequest{
			ShardId: "shard-id-B",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
		}

		pubsub.Subscribe(reqA, setter1)
		pubsub.Subscribe(reqA, setter2)
		pubsub.Subscribe(reqB, setter3)

		for i := 0; i < 10; i++ {
			pubsub.Publish(&loggregator_v2.Envelope{
				SourceId: "1",
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{},
				},
			})
		}

		Expect(setter1.envelopes).To(Or(HaveLen(0), HaveLen(10)))
		Expect(setter2.envelopes).To(Or(HaveLen(0), HaveLen(10)))
		Expect(len(setter1.envelopes)).ToNot(Equal(len(setter2.envelopes)))
		Expect(setter3.envelopes).To(HaveLen(10))
	})

	It("while sharding, sends like counters to the same subscription", func() {
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()
		setter3 := newSpyDataSetter()

		reqA := &loggregator_v2.EgressBatchRequest{
			ShardId:           "shard-id-A",
			DeterministicName: "black",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
			},
		}
		reqB := &loggregator_v2.EgressBatchRequest{
			ShardId: "shard-id-B",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
			},
		}
		reqC := &loggregator_v2.EgressBatchRequest{
			ShardId:           "shard-id-A",
			DeterministicName: "blue",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Counter{
						Counter: &loggregator_v2.CounterSelector{},
					},
				},
			},
		}

		pubsub.Subscribe(reqA, setter1)
		pubsub.Subscribe(reqC, setter2)
		pubsub.Subscribe(reqB, setter3)

		// "hash" and "other-hash" hash to different values % 2
		for i := 0; i < 10; i++ {
			pubsub.Publish(&loggregator_v2.Envelope{
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name: "hash",
					},
				},
			})

			pubsub.Publish(&loggregator_v2.Envelope{
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name: "other-hash",
					},
				},
			})
		}

		Expect(setter1.envelopes).To(HaveLen(10))
		Expect(setter2.envelopes).To(HaveLen(10))
		Expect(setter3.envelopes).To(HaveLen(20))

		// Counters
		Expect(setter1.envelopes[0].GetCounter().GetName()).To(Or(
			Equal("hash"),
			Equal("other-hash"),
		))
		Expect(setter2.envelopes[0].GetCounter().GetName()).To(Or(
			Equal("hash"),
			Equal("other-hash"),
		))
		Expect(setter1.envelopes[0].GetCounter().GetName()).ToNot(Equal(setter2.envelopes[0].GetCounter().GetName()))
	})

	It("while sharding, sends like gauges to the same subscription", func() {
		setter1 := newSpyDataSetter()
		setter2 := newSpyDataSetter()
		setter3 := newSpyDataSetter()

		reqA := &loggregator_v2.EgressBatchRequest{
			ShardId:           "shard-id-A",
			DeterministicName: "black",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{},
					},
				},
			},
		}
		reqB := &loggregator_v2.EgressBatchRequest{
			ShardId: "shard-id-B",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{},
					},
				},
			},
		}
		reqC := &loggregator_v2.EgressBatchRequest{
			ShardId:           "shard-id-A",
			DeterministicName: "blue",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{},
					},
				},
			},
		}

		pubsub.Subscribe(reqA, setter1)
		pubsub.Subscribe(reqC, setter2)
		pubsub.Subscribe(reqB, setter3)

		// "hash" and "other-hash" hash to different values % 2
		for i := 0; i < 10; i++ {
			pubsub.Publish(&loggregator_v2.Envelope{
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"a": {},
							"b": {},
						},
					},
				},
			})

			pubsub.Publish(&loggregator_v2.Envelope{
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"a": {},
						},
					},
				},
			})
		}

		Expect(setter1.envelopes).To(HaveLen(10))
		Expect(setter2.envelopes).To(HaveLen(10))
		Expect(setter3.envelopes).To(HaveLen(20))

		// Gauges
		Expect(setter1.envelopes[0].GetGauge().GetMetrics()).To(Or(
			HaveKey("b"),
			Not(HaveKey("b")),
		))
		Expect(setter2.envelopes[0].GetGauge().GetMetrics()).To(Or(
			HaveKey("b"),
			Not(HaveKey("b")),
		))
		Expect(setter1.envelopes[0].GetGauge().GetMetrics()).ToNot(Equal(setter2.envelopes[0].GetGauge().GetMetrics()))
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

		It("selects no types with no envelope type selector", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{SourceId: "some-id"},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			publishAllTypes(pubsub, "some-id")

			Expect(setter.envelopes).To(HaveLen(0))
		})

		It("selects counters when given a name", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Counter{
							Counter: &loggregator_v2.CounterSelector{
								Name: "a",
							},
						},
					},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)
			pubsub.Publish(buildCounter("some-id", "a"))
			pubsub.Publish(buildCounter("some-id", "b"))

			Expect(setter.envelopes).To(HaveLen(1))
		})

		It("selects gauges when given names", func() {
			req := &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Gauge{
							Gauge: &loggregator_v2.GaugeSelector{
								Names: []string{"a", "b"},
							},
						},
					},
				},
			}
			setter := newSpyDataSetter()

			pubsub.Subscribe(req, setter)

			pubsub.Publish(&loggregator_v2.Envelope{
				SourceId: "some-id",
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"a": {
								Unit:  "bytes",
								Value: 12342.23,
							},
						},
					},
				},
			})

			pubsub.Publish(&loggregator_v2.Envelope{
				SourceId: "some-id",
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"a": {
								Unit:  "bytes",
								Value: 12342.23,
							},
							"b": {
								Unit:  "bytes",
								Value: 12342.23,
							},
						},
					},
				},
			})

			pubsub.Publish(&loggregator_v2.Envelope{
				SourceId: "some-id",
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"a": {
								Unit:  "bytes",
								Value: 12342.23,
							},
							"b": {
								Unit:  "bytes",
								Value: 12342.23,
							},
							"c": {
								Unit:  "bytes",
								Value: 12342.23,
							},
						},
					},
				},
			})

			Expect(setter.envelopes).To(HaveLen(1))
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
					{
						SourceId: "a",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
					{
						SourceId: "a",
						Message: &loggregator_v2.Selector_Counter{
							Counter: &loggregator_v2.CounterSelector{},
						},
					},
				},
			}

			setter1 := newSpyDataSetter()
			setter2 := newSpyDataSetter()

			pubsub.Subscribe(req1, setter1)
			pubsub.Subscribe(req2, setter2)

			publishAllTypes(pubsub, "a")
			publishAllTypes(pubsub, "b")

			Expect(setter1.envelopes).To(HaveLen(2))
			Expect(setter2.envelopes).To(HaveLen(2))
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
	pubsub.Publish(buildCounter(id, "my-time"))
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

func buildCounter(id, name string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId: id,
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  name,
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
