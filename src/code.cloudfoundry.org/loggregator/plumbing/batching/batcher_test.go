package batching_test

import (
	"time"

	"code.cloudfoundry.org/loggregator/plumbing/batching"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Batcher", func() {
	It("honors the batch size when writing", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(1, time.Minute, writer)

		b.Write("item")

		Expect(writer.batch).To(HaveLen(1))
		Expect(writer.batch[0]).To(Equal("item"))
	})

	It("resets the internal buffer after writes", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(1, time.Minute, writer)

		b.Write("item")

		Expect(writer.batch).To(HaveLen(1))
		Expect(writer.batch[0]).To(Equal("item"))

		// Ensure it resets the batch each time
		b.Write("other-item")

		Expect(writer.batch).To(HaveLen(1))
		Expect(writer.batch[0]).To(Equal("other-item"))
	})

	It("honors the time interval when writing", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Minute, writer)

		b.Write("item")

		Expect(writer.batch).To(HaveLen(0))
	})

	It("writes a partial batch when the interval has lapsed", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(10, 1*time.Millisecond, writer)

		b.Write("item")

		// Wait for interval to lapse
		time.Sleep(10 * time.Millisecond)
		b.Write("other-item")

		Expect(writer.batch).To(HaveLen(2))
	})

	It("flushes a partial batch if the interval has lapsed", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Nanosecond, writer)

		// Wait for interval to lapse
		time.Sleep(time.Millisecond)

		b.Write("item")
		b.Flush()

		Expect(writer.batch).To(HaveLen(1))

		// Ensure the interval is reset after a write
		writer.called = 0
		b.Flush()
		Expect(writer.called).To(Equal(0))
	})

	It("honors the time interval when flushing", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Minute, writer)

		b.Write("item")
		b.Flush()

		Expect(writer.called).To(Equal(0))
	})

	It("avoids calling the writer with an empty batch", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Nanosecond, writer)

		// Wait for interval to lapse
		time.Sleep(time.Millisecond)

		b.Flush()

		Expect(writer.called).To(Equal(0))
	})

	It("provides a means to guarantee a flushed write", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Second, writer)

		b.Write("item")
		b.ForcedFlush()

		Expect(writer.called).To(Equal(1))
	})

	It("avoids writing an empty batch on forced flush", func() {
		writer := &spyWriter{}
		b := batching.NewBatcher(2, time.Second, writer)

		b.ForcedFlush()

		Expect(writer.called).To(Equal(0))
	})
})

type spyWriter struct {
	batch  []interface{}
	called int
}

func (w *spyWriter) Write(batch []interface{}) {
	w.batch = batch
	w.called++
}
