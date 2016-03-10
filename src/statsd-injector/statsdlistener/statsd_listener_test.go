package statsdlistener_test

import (
	"net"
	"statsd-injector/statsdlistener"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("StatsdListener", func() {
	Describe("Run", func() {

		BeforeEach(func() {
			loggertesthelper.TestLoggerSink.Clear()
		})

		It("reads multiple gauges (on different lines) in the same packet", func(done Done) {
			listener := statsdlistener.New(51162, loggertesthelper.Logger())

			envelopeChan := make(chan *events.Envelope)

			wg := stopMeLater(func() { listener.Run(envelopeChan) })

			defer func() {
				stopAndWait(func() { listener.Stop() }, wg)
				close(done)
			}()

			Eventually(loggertesthelper.TestLoggerSink.LogContents).Should(ContainSubstring("Listening for statsd on host"))

			connection, err := net.Dial("udp", ":51162")
			Expect(err).ToNot(HaveOccurred())
			defer connection.Close()
			statsdmsg := []byte("fake-origin.test.gauge:23|g\nfake-origin.other.thing:42|g\nfake-origin.sampled.gauge:17.5|g|@0.2")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.gauge", 23, "gauge")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "other.thing", 42, "gauge")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "sampled.gauge", 87.5, "gauge")
		}, 5)

		It("processes gauge increment/decrement stats", func(done Done) {
			listener := statsdlistener.New(51162, loggertesthelper.Logger())

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
			statsdmsg := []byte("fake-origin.test.gauge:23|g\nfake-origin.test.gauge:+7|g\nfake-origin.test.gauge:-5|g")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.gauge", 23, "gauge")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.gauge", 30, "gauge")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.gauge", 25, "gauge")
		})

		It("reads multiple timings (on different lines) in the same packet", func(done Done) {
			listener := statsdlistener.New(51162, loggertesthelper.Logger())

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
			statsdmsg := []byte("fake-origin.test.timing:23|ms\nfake-origin.other.thing:420|ms\nfake-origin.sampled.timing:71|ms|@0.1")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.timing", 23, "ms")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "other.thing", 420, "ms")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "sampled.timing", 710, "ms")
		}, 5)

		It("reads multiple counters (on different lines) in the same packet", func(done Done) {
			listener := statsdlistener.New(51162, loggertesthelper.Logger())

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
			statsdmsg := []byte("fake-origin.test.counter:23|c\nfake-origin.other.thing:420|c\nfake-origin.sampled.counter:71|c|@0.1")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.counter", 23, "counter")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "other.thing", 420, "counter")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "sampled.counter", 710, "counter")
		}, 5)

		It("processes counter increment/decrement stats", func(done Done) {
			listener := statsdlistener.New(51162, loggertesthelper.Logger())

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
			statsdmsg := []byte("fake-origin.test.counter:23|c\nfake-origin.test.counter:+7|c\nfake-origin.test.counter:-5|c")
			_, err = connection.Write(statsdmsg)
			Expect(err).ToNot(HaveOccurred())

			var receivedEnvelope *events.Envelope

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.counter", 23, "counter")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.counter", 30, "counter")

			Eventually(envelopeChan).Should(Receive(&receivedEnvelope))
			checkValueMetric(receivedEnvelope, "fake-origin", "test.counter", 25, "counter")
		})
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

func checkValueMetric(receivedEnvelope *events.Envelope, origin string, name string, value float64, unit string) {
	Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ValueMetric))
	Expect(receivedEnvelope.GetOrigin()).To(Equal(origin))

	vm := receivedEnvelope.GetValueMetric()
	Expect(vm.GetName()).To(Equal(name))
	Expect(vm.GetValue()).To(BeNumerically("==", value))
	Expect(vm.GetUnit()).To(Equal(unit))

}
