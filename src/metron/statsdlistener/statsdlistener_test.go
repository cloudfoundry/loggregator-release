package statsdlistener_test

import (
	"metron/statsdlistener"

	"net"
	"sync"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	"github.com/cloudfoundry/dropsonde/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StatsdListener", func() {
	Describe("Run", func() {

		BeforeEach(func() {
			loggertesthelper.TestLoggerSink.Clear()
		})

		It("reads multiple gauges (on different lines) in the same packet", func(done Done) {
			listener := statsdlistener.NewStatsdListener("localhost:51162", loggertesthelper.Logger(), "name")

			envelopeChan := make(chan *events.Envelope)

			wg := stopMeLater(func() { listener.Run(envelopeChan) })

			defer func() {
				stopAndWait(func() { listener.Stop() }, wg)
				close(done)
			}()

			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening for statsd on host"))

			connection, err := net.Dial("udp", "localhost:51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()
			statsdmsg := []byte("fake-origin.test.gauge:23|g\nfake-origin.other.thing:42|g")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope
			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(receivedEnvelope.GetOrigin()).To(Equal("fake-origin"))

			vm := receivedEnvelope.GetValueMetric()
			Expect(vm.GetName()).To(Equal("test.gauge"))
			Expect(vm.GetValue()).To(BeNumerically("==", 23))
			Expect(vm.GetUnit()).To(Equal("gauge"))

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(receivedEnvelope.GetOrigin()).To(Equal("fake-origin"))

			vm = receivedEnvelope.GetValueMetric()
			Expect(vm.GetName()).To(Equal("other.thing"))
			Expect(vm.GetValue()).To(BeNumerically("==", 42))
			Expect(vm.GetUnit()).To(Equal("gauge"))
		}, 5)
	})
})

func stopMeLater(f func()) *sync.WaitGroup {
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		f()
		wg.Done()
	}()

	return &wg
}

func stopAndWait(f func(), wg *sync.WaitGroup) {
	f()
	wg.Wait()
}
