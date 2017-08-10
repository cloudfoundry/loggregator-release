package batching_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator/plumbing/batching"
)

var _ = Describe("ByteBatcher", func() {
	It("works", func() {
		writer := &spyByteWriter{}
		b := batching.NewByteBatcher(1, time.Minute, writer)

		b.Write([]byte("item"))

		Expect(writer.batch).To(HaveLen(1))
		Expect(writer.batch[0]).To(Equal([]byte("item")))
	})
})

type spyByteWriter struct {
	batch  [][]byte
	called int
}

func (w *spyByteWriter) Write(batch [][]byte) {
	w.batch = batch
	w.called++
}
