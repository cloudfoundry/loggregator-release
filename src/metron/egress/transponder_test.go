package egress_test

import (
	"metron/egress"
	v2 "plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transponder", func() {

	It("reads from the buffer to the writer", func() {
		envelope := &v2.Envelope{SourceUuid: "uuid"}
		nexter := newMockNexter()
		nexter.NextOutput.Ret0 <- envelope
		writer := newMockWriter()
		close(writer.WriteOutput.Ret0)
		tx := egress.NewTransponder(nexter, writer)

		go tx.Start()

		Eventually(nexter.NextCalled).Should(Receive())
		Eventually(writer.WriteInput.Msg).Should(Receive(Equal(envelope)))
	})
})
