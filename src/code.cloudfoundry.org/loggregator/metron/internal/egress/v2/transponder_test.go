package v2_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transponder", func() {
	It("reads from the buffer to the writer", func() {
		envelope := &v2.Envelope{SourceId: "uuid"}
		nexter := newMockNexter()
		nexter.TryNextOutput.Ret0 <- envelope
		nexter.TryNextOutput.Ret1 <- true
		writer := newMockWriter()
		close(writer.WriteOutput.Ret0)

		tx := egress.NewTransponder(nexter, writer, nil, 1, time.Nanosecond, testhelper.NewMetricClient())
		go tx.Start()

		Eventually(nexter.TryNextCalled).Should(Receive())
		Eventually(writer.WriteInput.Msg).Should(Receive(Equal([]*v2.Envelope{envelope})))
	})

	Describe("batching", func() {
		It("emits once the batch count has been reached", func() {
			envelope := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, testhelper.NewMetricClient())
			go tx.Start()

			var batch []*v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&batch))
			Expect(batch).To(HaveLen(5))
		})

		It("emits once the batch interval has been reached", func() {
			envelope := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			nexter.TryNextOutput.Ret0 <- envelope
			nexter.TryNextOutput.Ret1 <- true
			close(nexter.TryNextOutput.Ret0)
			close(nexter.TryNextOutput.Ret1)

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Millisecond, testhelper.NewMetricClient())
			go tx.Start()

			var batch []*v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&batch))
			Expect(batch).To(HaveLen(1))
		})

		It("clears batch upon egress failure", func() {
			envelope := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()

			go func() {
				for {
					writer.WriteOutput.Ret0 <- errors.New("some-error")
				}
			}()

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, testhelper.NewMetricClient())
			go tx.Start()

			Eventually(writer.WriteCalled).Should(HaveLen(1))
			Consistently(writer.WriteCalled).Should(HaveLen(1))
		})

		It("emits egress metric", func() {
			envelope := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			for i := 0; i < 6; i++ {
				nexter.TryNextOutput.Ret0 <- envelope
				nexter.TryNextOutput.Ret1 <- true
			}

			spy := testhelper.NewMetricClient()
			tx := egress.NewTransponder(nexter, writer, nil, 5, time.Minute, spy)
			go tx.Start()

			f := func() uint64 {
				return spy.GetDelta("egress")
			}

			Eventually(f).Should(Equal(uint64(5)))
		})
	})

	Describe("tagging", func() {
		It("adds the given tags to all envelopes", func() {
			tags := map[string]string{
				"tag-one": "value-one",
				"tag-two": "value-two",
			}
			input := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))

			Expect(output).To(HaveLen(1))
			Expect(output[0].DeprecatedTags["tag-one"].GetText()).To(Equal("value-one"))
			Expect(output[0].DeprecatedTags["tag-two"].GetText()).To(Equal("value-two"))
		})

		It("does not write over tags if they already exist", func() {
			tags := map[string]string{
				"existing-tag": "some-new-value",
			}
			input := &v2.Envelope{
				SourceId: "uuid",
				DeprecatedTags: map[string]*v2.Value{
					"existing-tag": {
						Data: &v2.Value_Text{
							Text: "existing-value",
						},
					},
				},
			}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))
			Expect(output).To(HaveLen(1))

			Expect(output[0].DeprecatedTags["existing-tag"].GetText()).To(Equal("existing-value"))
		})

		// This is required for backwards compatibility purposes.
		// It should be removed once DeprecatedTags are removed.
		It("moves Tags to DeprecatedTags", func() {
			input := &v2.Envelope{
				SourceId: "uuid",
				Tags: map[string]string{
					"non-deprecated-tag": "some-new-value",
				},
			}
			nexter := newMockNexter()
			nexter.TryNextOutput.Ret0 <- input
			nexter.TryNextOutput.Ret1 <- true
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, nil, 1, time.Nanosecond, testhelper.NewMetricClient())

			go tx.Start()

			Eventually(nexter.TryNextCalled).Should(Receive())

			var output []*v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))
			Expect(output).To(HaveLen(1))

			Expect(output[0].DeprecatedTags["non-deprecated-tag"].GetText()).To(Equal("some-new-value"))

		})
	})
})
