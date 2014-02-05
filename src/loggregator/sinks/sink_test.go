package sinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinks"
	"runtime"
)

var _ = Describe("SyslogSink", func() {
	It("should only allow one request to close", func(done Done) {
		closeChan := make(chan sinks.Sink, 1)

		sink := testSink{"1", closeChan}
		go sink.Run()
		runtime.Gosched()

		closeSink := <-closeChan
		Expect(&sink).To(Equal(closeSink))

		Expect(closeChan).To(BeEmpty())
		close(done)
	})
})
