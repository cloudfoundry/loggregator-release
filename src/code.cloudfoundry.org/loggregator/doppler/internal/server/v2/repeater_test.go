package v2_test

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/server/v2"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
	. "github.com/onsi/ginkgo"
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
