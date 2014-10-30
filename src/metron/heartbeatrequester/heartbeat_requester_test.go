package heartbeatrequester_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/dropsonde/control"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"metron/heartbeatrequester"
	"net"
	"sync"
	"time"
)

var _ = Describe("PingSender", func() {
	var (
		heartbeatRequester     *heartbeatrequester.HeartbeatRequester
		pingConn               net.PacketConn
		pingers                sync.WaitGroup
		startHeartbeatRequests func(target net.Addr)
	)

	BeforeEach(func() {
		heartbeatRequester = heartbeatrequester.NewHeartbeatRequester(40 * time.Millisecond)
		pingConn, _ = net.ListenPacket("udp4", "")
		startHeartbeatRequests = func(target net.Addr) {
			pingers.Add(1)
			heartbeatRequester.Start(target, pingConn)
			pingers.Done()
		}
	})

	AfterEach(func() {
		pingers.Wait()
		heartbeatRequester = nil
	})

	It("StartHeartbeartRequests sends 4-5 unanswered heartbeat requests to a target before timing out", func(done Done) {
		pingTarget, err := net.ListenPacket("udp4", "")
		Expect(err).NotTo(HaveOccurred())

		go startHeartbeatRequests(pingTarget.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget.LocalAddr())

		var expectPingTimeout = func() {
			messageSlice := make([]byte, 65535)
			readCount := 0
			for {
				pingTarget.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				n, _, err := pingTarget.ReadFrom(messageSlice)
				if err != nil {
					break
				}
				var controlMessage control.ControlMessage
				err = proto.Unmarshal(messageSlice[0:n], &controlMessage)
				Expect(err).NotTo(HaveOccurred())
				Expect(controlMessage.GetControlType()).To(Equal(control.ControlMessage_HeartbeatRequest))
				readCount++
			}
			Expect(readCount).To(BeNumerically(">=", 4))
			Expect(readCount).To(BeNumerically("<", 6))
		}

		expectPingTimeout()

		// start ping with the same address should reinitiate the cycle
		heartbeatRequester.Start(pingTarget.LocalAddr(), pingConn)

		expectPingTimeout()
		close(done)
	})

	It("pings a known sender periodically", func() {
		pingTarget, err := net.ListenPacket("udp4", "")
		Expect(err).NotTo(HaveOccurred())

		go startHeartbeatRequests(pingTarget.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget.LocalAddr())

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

		go startHeartbeatRequests(pingTarget1.LocalAddr())
		go startHeartbeatRequests(pingTarget2.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget1.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget2.LocalAddr())

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

		go startHeartbeatRequests(pingTarget.LocalAddr())
		go startHeartbeatRequests(pingTarget.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget.LocalAddr())

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

		go startHeartbeatRequests(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget.ReadFrom(messageSlice)

		heartbeatRequester.Stop(pingTarget.LocalAddr())

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

		go startHeartbeatRequests(pingTarget1.LocalAddr())
		go startHeartbeatRequests(pingTarget2.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget1.ReadFrom(messageSlice)

		heartbeatRequester.Stop(pingTarget1.LocalAddr())

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
		heartbeatRequester.Stop(pingTarget2.LocalAddr())
	})

	It("restarts pinging a target that was previously stopped", func() {
		pingTarget, _ := net.ListenPacket("udp4", "")

		go startHeartbeatRequests(pingTarget.LocalAddr())

		messageSlice := make([]byte, 5)
		pingTarget.ReadFrom(messageSlice)

		heartbeatRequester.Stop(pingTarget.LocalAddr())

		pingers.Wait()

		go startHeartbeatRequests(pingTarget.LocalAddr())
		defer heartbeatRequester.Stop(pingTarget.LocalAddr())

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

	It("does not panic if Stop an unknown target", func() {
		Expect(func() { heartbeatRequester.Stop(fakeAddr{}) }).NotTo(Panic())
	})
})

type fakeAddr struct{}

func (f fakeAddr) String() string {
	return "string"
}

func (f fakeAddr) Network() string {
	return "network"
}
