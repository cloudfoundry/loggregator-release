package component_test

import (
	"fmt"
	"integration_tests"
	"metron/testutil"
	"net"
	"plumbing"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metron", func() {
	Context("when a doppler is accepting gRPC connections", func() {
		var (
			metronCleanup func()
			metronPort    int
			dopplerServer *testutil.Server
			eventEmitter  dropsonde.EventEmitter
		)

		BeforeEach(func() {
			var err error
			dopplerServer, err = testutil.NewServer()
			Expect(err).ToNot(HaveOccurred())

			var metronReady func()
			metronCleanup, metronPort, metronReady = integration_tests.SetupMetron("localhost", dopplerServer.Port(), 0)
			defer metronReady()

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")
		})

		AfterEach(func() {
			dopplerServer.Stop()
			metronCleanup()
		})

		It("writes to the doppler via gRPC", func() {
			emitEnvelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			f := func() int {
				eventEmitter.Emit(emitEnvelope)
				return len(dopplerServer.PusherCalled)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))

			var rx plumbing.DopplerIngestor_PusherServer
			Expect(dopplerServer.PusherInput.Arg0).Should(Receive(&rx))

			data, err := rx.Recv()
			Expect(err).ToNot(HaveOccurred())

			envelope := new(events.Envelope)
			Expect(envelope.Unmarshal(data.Payload)).To(Succeed())
		})
	})

	Context("when the doppler is only accepting UDP messages", func() {
		var (
			metronCleanup  func()
			dopplerCleanup func()
			metronPort     int
			udpPort        int
			eventEmitter   dropsonde.EventEmitter
			dopplerConn    *net.UDPConn
		)

		BeforeEach(func() {
			addr, err := net.ResolveUDPAddr("udp4", "localhost:0")
			Expect(err).ToNot(HaveOccurred())
			dopplerConn, err = net.ListenUDP("udp4", addr)
			Expect(err).ToNot(HaveOccurred())
			udpPort = HomeAddrToPort(dopplerConn.LocalAddr())
			dopplerCleanup = func() {
				dopplerConn.Close()
			}

			var metronReady func()
			metronCleanup, metronPort, metronReady = integration_tests.SetupMetron("localhost", 0, udpPort)
			defer metronReady()

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")
		})

		AfterEach(func() {
			dopplerCleanup()
			metronCleanup()
		})

		It("writes to the doppler via UDP", func() {
			envelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			c := make(chan bool, 100)
			var wg sync.WaitGroup
			wg.Add(1)
			defer wg.Wait()
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				buffer := make([]byte, 1024)
				for {
					dopplerConn.SetReadDeadline(time.Now().Add(5 * time.Second))
					_, err := dopplerConn.Read(buffer)
					Expect(err).ToNot(HaveOccurred())

					select {
					case c <- true:
					default:
						return
					}
				}
			}()

			f := func() int {
				eventEmitter.Emit(envelope)
				return len(c)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))
		})
	})
})

func HomeAddrToPort(addr net.Addr) int {
	port, err := strconv.Atoi(strings.Replace(addr.String(), "127.0.0.1:", "", 1))
	if err != nil {
		panic(err)
	}
	return port
}
