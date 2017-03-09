package ingress_test

import (
	v2 "plumbing/v2"
	"rlp/internal/ingress"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RequestConverter", func() {
	var (
		c ingress.RequestConverter
	)

	BeforeEach(func() {
		c = ingress.NewRequestConverter()
	})

	It("sets the appID", func() {
		req := c.Convert(&v2.EgressRequest{
			Filter: &v2.Filter{
				SourceId: "some-id",
			},
		})

		Expect(req.GetFilter().AppID).To(Equal("some-id"))
	})

	It("sets the shardID", func() {
		req := c.Convert(&v2.EgressRequest{
			ShardId: "some-id",
		})

		Expect(req.ShardID).To(Equal("some-id"))
	})

	It("sets a LogFilter", func() {
		req := c.Convert(&v2.EgressRequest{
			Filter: &v2.Filter{
				Message: &v2.Filter_Log{
					Log: &v2.LogFilter{},
				},
			},
		})

		Expect(req.GetFilter().GetLog()).ToNot(BeNil())
	})

	It("sets a CounterFilter", func() {
		req := c.Convert(&v2.EgressRequest{
			Filter: &v2.Filter{
				Message: &v2.Filter_Counter{
					Counter: &v2.CounterFilter{},
				},
			},
		})

		Expect(req.GetFilter().GetCounter()).ToNot(BeNil())
	})

	It("sets a GaugeFilter", func() {
		req := c.Convert(&v2.EgressRequest{
			Filter: &v2.Filter{
				Message: &v2.Filter_Gauge{
					Gauge: &v2.GaugeFilter{},
				},
			},
		})

		Expect(req.GetFilter().GetGauge()).ToNot(BeNil())
	})

	It("sets a TimerFilter", func() {
		req := c.Convert(&v2.EgressRequest{
			Filter: &v2.Filter{
				Message: &v2.Filter_Timer{
					Timer: &v2.TimerFilter{},
				},
			},
		})

		Expect(req.GetFilter().GetTimer()).ToNot(BeNil())
	})
})
