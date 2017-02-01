package component_test

import (
	"context"
	"metron/config"
	"metron/testutil"
	v2 "plumbing/v2"
	"testservers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetronAggregator", func() {
	var (
		metronCleanup  func()
		metronConfig   config.Config
		consumerServer *testutil.Server
	)

	BeforeEach(func() {
		var err error
		consumerServer, err = testutil.NewServer()
		Expect(err).ToNot(HaveOccurred())

		var metronReady func()
		metronConfig = testservers.BuildMetronConfig("localhost", consumerServer.Port(), 0)
		metronConfig.MetricBatchIntervalMilliseconds = 10000
		metronCleanup, metronConfig, metronReady = testservers.StartMetron(
			metronConfig,
		)
		defer metronReady()
	})

	AfterEach(func() {
		consumerServer.Stop()
		metronCleanup()
	})

	It("calculates totals for counter envelopes", func() {
		client := metronClient(metronConfig)
		ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
		sender, err := client.Sender(ctx)
		Expect(err).ToNot(HaveOccurred())

		Consistently(func() error {
			return sender.Send(buildCounterEnvelope(10, "name-1", "origin-1"))
		}, 2).Should(Succeed())

		var rx v2.DopplerIngress_SenderServer
		Expect(consumerServer.V2.SenderInput.Arg0).Should(Receive(&rx))

		f := func() uint64 {
			envelope, err := rx.Recv()
			Expect(err).ToNot(HaveOccurred())

			if envelope.GetCounter().Name != "name-1" {
				return 0
			}

			return envelope.GetCounter().GetTotal()
		}
		Eventually(f, 10).Should(BeNumerically(">", 40))
	})
})

func buildCounterEnvelope(delta uint64, name, origin string) *v2.Envelope {
	return &v2.Envelope{
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: name,
				Value: &v2.Counter_Delta{
					Delta: delta,
				},
			},
		},
		Tags: map[string]*v2.Value{
			"origin": {Data: &v2.Value_Text{origin}},
		},
	}
}
