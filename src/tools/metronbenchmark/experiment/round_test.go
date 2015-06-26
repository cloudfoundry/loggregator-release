package experiment_test

import (
	"time"
	"tools/metronbenchmark/experiment"
	"tools/metronbenchmark/messagewriter"

	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Round", func() {
	Describe("Perform", func() {
		It("sends messages at the given rate for the duration", func(done Done) {
			r := experiment.NewRound(42, 100, 500*time.Millisecond)

			messages := make(chan *events.LogMessage, 1000)
			writer := messagewriter.NewChannelLogWriter(messages)

			r.Perform(writer)

			Expect(messages).To(HaveLen(50))
			close(done)
		})
	})
})
