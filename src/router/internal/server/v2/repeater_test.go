package v2_test

import (
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	v2 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repeater", func() {
	It("reads passes the result of next to the sender", func() {
		nextFn := func() *loggregator_v2.Envelope {
			return &loggregator_v2.Envelope{
				SourceId: "some-source-id",
			}
		}
		sendStream := make(chan *loggregator_v2.Envelope)
		sendFn := func(e *loggregator_v2.Envelope) {
			sendStream <- e
		}
		r := v2.NewRepeater(sendFn, nextFn)

		go r.Start()

		expected := &loggregator_v2.Envelope{
			SourceId: "some-source-id",
		}
		var actual *loggregator_v2.Envelope
		Eventually(sendStream).Should(Receive(&actual))
		Expect(actual).To(Equal(expected))
	})
})
