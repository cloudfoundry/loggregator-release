package pingsender_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/pingsender"
	"net"
	"sync"
	"time"
)

var _ = Describe("PingSender", func() {
	var (
		pingSender   *pingsender.PingSender
		pingConn     net.PacketConn
		pingers      sync.WaitGroup
		startPinging func(target net.Addr)
	)

	BeforeEach(func() {
		pingSender = pingsender.NewPingSender(40 * time.Millisecond)
		pingConn, _ = net.ListenPacket("udp4", "")
		startPinging = func(target net.Addr) {
			pingers.Add(1)
			pingSender.StartPing(target, pingConn)
			pingers.Done()
		}
	})

	AfterEach(func() {
		pingers.Wait()
		pingSender = nil
	})

	It("Start ping sends 4-5 unanswered pings to a target before timing out", func(done Done) {
		pingTarget, err := net.ListenPacket("udp4", "")
		Expect(err).NotTo(HaveOccurred())

		go startPinging(pingTarget.LocalAddr())
		defer pingSender.StopPing(pingTarget.LocalAddr())

		var expectPingTimeout = func() {
			messageSlice := make([]byte, 5)
			readCount := 0
			for {
				pingTarget.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				_, _, err = pingTarget.ReadFrom(messageSlice)
				if err != nil {
					break
				}
				Expect(string(messageSlice)).To(ContainSubstring("Ping"))
				readCount++
			}
			Expect(readCount).To(BeNumerically(">=", 4))
			Expect(readCount).To(BeNumerically("<", 6))
		}

		expectPingTimeout()

		// start ping with the same address should reinitiate the cycle
		pingSender.StartPing(pingTarget.LocalAddr(), pingConn)

		expectPingTimeout()
		close(done)
	})

	It("pings a known sender periodically", func() {
		pingTarget, err := net.ListenPacket("udp4", "")
		Expect(err).NotTo(HaveOccurred())

		go startPinging(pingTarget.LocalAddr())
		defer pingSender.StopPing(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, _, err = pingTarget.ReadFrom(messageSlice)
		Expect(err).NotTo(HaveOccurred())

		pingTarget.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, _, err = pingTarget.ReadFrom(messageSlice)
		Expect(err).NotTo(HaveOccurred())
	})

	It("pings all known senders", func() {
		pingTarget1, _ := net.ListenPacket("udp4", "")
		pingTarget2, _ := net.ListenPacket("udp4", "")

		go startPinging(pingTarget1.LocalAddr())
		go startPinging(pingTarget2.LocalAddr())
		defer pingSender.StopPing(pingTarget1.LocalAddr())
		defer pingSender.StopPing(pingTarget2.LocalAddr())

		pingTarget1.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		pingTarget2.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

		messageSlice := make([]byte, 5)
		_, _, err := pingTarget1.ReadFrom(messageSlice)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = pingTarget2.ReadFrom(messageSlice)
		Expect(err).NotTo(HaveOccurred())
	})

	It("only pings target once per interval", func() {
		pingTarget, _ := net.ListenPacket("udp4", "")

		go startPinging(pingTarget.LocalAddr())
		go startPinging(pingTarget.LocalAddr())
		defer pingSender.StopPing(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		messageCount := 0
		timer := time.NewTimer(79 * time.Millisecond)
	loop:
		for {
			select {
			case <-timer.C:
				break loop
			default:
				pingTarget.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				_, _, err := pingTarget.ReadFrom(messageSlice)
				if err == nil {
					messageCount++
				}
			}
		}

		Expect(messageCount).To(Equal(1))
	})

	It("eventually stops sending pings asked to stop", func() {
		pingTarget, _ := net.ListenPacket("udp4", "")

		go startPinging(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget.ReadFrom(messageSlice)

		pingSender.StopPing(pingTarget.LocalAddr())

		messageCount := 0
		timer := time.NewTimer(100 * time.Millisecond)
	loop:
		for {
			select {
			case <-timer.C:
				break loop
			default:
				pingTarget.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				_, _, err := pingTarget.ReadFrom(messageSlice)
				if err == nil {
					messageCount++
				}
			}
		}

		Expect(messageCount).To(BeNumerically("<=", 1))
	})

	It("only stops sending pings to the given target", func() {
		pingTarget1, _ := net.ListenPacket("udp4", "")
		pingTarget2, _ := net.ListenPacket("udp4", "")

		go startPinging(pingTarget1.LocalAddr())
		go startPinging(pingTarget2.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget1.ReadFrom(messageSlice)

		pingSender.StopPing(pingTarget1.LocalAddr())

		messageCount := 0
		timer := time.NewTimer(100 * time.Millisecond)

	loop:
		for {
			select {
			case <-timer.C:
				break loop
			default:
				pingTarget2.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				_, _, err := pingTarget2.ReadFrom(messageSlice)

				if err == nil {
					messageCount++
				}
			}
		}

		Expect(messageCount).To(BeNumerically(">=", 2))
		pingSender.StopPing(pingTarget2.LocalAddr())
	})

	It("restarts pinging a target that was previously stopped", func() {
		pingTarget, _ := net.ListenPacket("udp4", "")

		go startPinging(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget.ReadFrom(messageSlice)

		pingSender.StopPing(pingTarget.LocalAddr())

		pingers.Wait()

		go startPinging(pingTarget.LocalAddr())
		defer pingSender.StopPing(pingTarget.LocalAddr())

		messageCount := 0
		timer := time.NewTimer(50 * time.Millisecond)
	loop:
		for {
			select {
			case <-timer.C:
				break loop
			default:
				pingTarget.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
				_, _, err := pingTarget.ReadFrom(messageSlice)
				if err == nil {
					messageCount++
				}
			}
		}

		Expect(messageCount).To(BeNumerically(">=", 1))
	})

	It("does not panic if stopping an unknown target", func() {
		Expect(func() { pingSender.StopPing(fakeAddr{}) }).NotTo(Panic())
	})
})

type fakeAddr struct{}

func (f fakeAddr) String() string {
	return "string"
}

func (f fakeAddr) Network() string {
	return "network"
}
