package eventlistener_test

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/gosteno"
	"metron/eventlistener"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

var _ = Describe("EventListener", func() {
	Context("with a listener running", func() {
		var listener eventlistener.EventListener
		var dataChannel <-chan []byte
		var fakePinger *fakePingSender
		var listenerClosed chan struct{}

		BeforeEach(func() {
			listenerClosed = make(chan struct{})

			fakePinger = &fakePingSender{pingTargets: make(map[string]chan (struct{}))}
			listener, dataChannel = eventlistener.NewEventListener("127.0.0.1:3456", gosteno.NewLogger("TestLogger"), "eventListener", fakePinger)
			go func() {
				listener.Start()
				close(listenerClosed)
			}()
		})

		AfterEach(func() {
			listener.Stop()
			<-listenerClosed
			fakePinger.StopAll()
			fakePinger.Wait()
		})

		It("should listen to the socket", func(done Done) {
			expectedData := "Some Data"
			otherData := "More stuff"

			connection, err := net.Dial("udp", "localhost:3456")

			_, err = connection.Write([]byte(expectedData))
			Expect(err).NotTo(HaveOccurred())

			received := <-dataChannel
			Expect(string(received)).To(Equal(expectedData))

			_, err = connection.Write([]byte(otherData))
			Expect(err).NotTo(HaveOccurred())

			receivedAgain := <-dataChannel
			Expect(string(receivedAgain)).To(Equal(otherData))

			metrics := listener.Emit().Metrics
			Expect(metrics).To(HaveLen(3)) //make sure all expected metrics are present
			for _, metric := range metrics {
				switch metric.Name {
				case "currentBufferCount":
					Expect(metric.Value).To(Equal(0))
				case "receivedMessageCount":
					Expect(metric.Value).To(Equal(uint64(2)))
				case "receivedByteCount":
					Expect(metric.Value).To(Equal(uint64(19)))
				default:
					Fail(fmt.Sprintf("Got an invalid metric name: %s", metric.Name))
				}
			}
			close(done)
		}, 2)

		It("requests a heartbeat from the sender when it receives an event", func(done Done) {
			connection, err := net.Dial("udp", "localhost:3456")
			Expect(err).NotTo(HaveOccurred())

			_, err = connection.Write([]byte("some data"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool { return fakePinger.StartedFor(connection.LocalAddr().String()) }).Should(BeTrue())

			close(done)
		})
	})

	Describe("Emit", func() {
		It("uses the given name for the context", func() {
			pinger := fakePingSender{}
			listener, _ := eventlistener.NewEventListener("127.0.0.1:3456", gosteno.NewLogger("TestLogger"), "secretEventOrange", &pinger)
			context := listener.Emit()

			Expect(context.Name).To(Equal("secretEventOrange"))
		})
	})
})

type fakePingSender struct {
	pingTargets map[string]chan (struct{})
	sync.WaitGroup
	sync.Mutex
}

func (pinger *fakePingSender) StartPing(senderAddr net.Addr, connection net.PacketConn) {
	pinger.Add(1)
	pinger.Lock()
	_, targetKnown := pinger.pingTargets[senderAddr.String()]

	if targetKnown {
		defer pinger.Done()
		defer pinger.Unlock()
		return
	}

	stop := make(chan struct{})
	pinger.pingTargets[senderAddr.String()] = stop
	pinger.Unlock()

	<-stop
	defer pinger.Done()
	defer pinger.Unlock()
	pinger.Lock()
	delete(pinger.pingTargets, senderAddr.String())
}

func (pinger *fakePingSender) StartedFor(addr string) bool {
	pinger.Lock()
	defer pinger.Unlock()
	_, startedForAddr := pinger.pingTargets[addr]
	return startedForAddr
}

func (pinger *fakePingSender) StopAll() {

	pinger.Lock()
	defer pinger.Unlock()

	for _, closeChan := range pinger.pingTargets {
		close(closeChan)
	}
}
