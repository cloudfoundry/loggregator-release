package v2_test

import (
	"sync/atomic"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	v2 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FanoutWriter", func() {
	It("delivers all written envelopes to the underlying writer", func() {
		const total = 200
		var count atomic.Int64
		spy := func(_ *loggregator_v2.Envelope) { count.Add(1) }

		f := v2.NewFanoutWriter(spy, 4)
		f.Start()

		for i := 0; i < total; i++ {
			f.Write(&loggregator_v2.Envelope{})
		}

		Eventually(func() int64 { return count.Load() }).Should(BeEquivalentTo(total))
	})

	It("Stop returns without blocking", func() {
		f := v2.NewFanoutWriter(func(*loggregator_v2.Envelope) {}, 2)
		f.Start()

		done := make(chan struct{})
		go func() { f.Stop(); close(done) }()
		Eventually(done).Should(BeClosed())
	})
})
