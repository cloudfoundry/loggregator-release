package v2_test

import (
	egress "metron/egress/v2"
	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transponder", func() {
	It("reads from the buffer to the writer", func() {
		envelope := &v2.Envelope{SourceId: "uuid"}
		nexter := newMockNexter()
		nexter.NextOutput.Ret0 <- envelope
		writer := newMockWriter()
		close(writer.WriteOutput.Ret0)

		tx := egress.NewTransponder(nexter, writer, nil)

		go tx.Start()

		Eventually(nexter.NextCalled).Should(Receive())
		Eventually(writer.WriteInput.Msg).Should(Receive(Equal(envelope)))
	})

	Describe("tagging", func() {
		It("adds the given tags to all envelopes", func() {
			tags := map[string]string{
				"tag-one": "value-one",
				"tag-two": "value-two",
			}
			input := &v2.Envelope{SourceId: "uuid"}
			nexter := newMockNexter()
			nexter.NextOutput.Ret0 <- input
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags)

			go tx.Start()

			Eventually(nexter.NextCalled).Should(Receive())

			var output *v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))

			Expect(output.Tags["tag-one"].GetText()).To(Equal("value-one"))
			Expect(output.Tags["tag-two"].GetText()).To(Equal("value-two"))
		})

		It("does not write over tags if they already exist", func() {
			tags := map[string]string{
				"existing-tag": "some-new-value",
			}
			input := &v2.Envelope{
				SourceId: "uuid",
				Tags: map[string]*v2.Value{
					"existing-tag": {
						Data: &v2.Value_Text{
							Text: "existing-value",
						},
					},
				},
			}
			nexter := newMockNexter()
			nexter.NextOutput.Ret0 <- input
			writer := newMockWriter()
			close(writer.WriteOutput.Ret0)

			tx := egress.NewTransponder(nexter, writer, tags)

			go tx.Start()

			Eventually(nexter.NextCalled).Should(Receive())

			var output *v2.Envelope
			Eventually(writer.WriteInput.Msg).Should(Receive(&output))

			Expect(output.Tags["existing-tag"].GetText()).To(Equal("existing-value"))
		})
	})
})
