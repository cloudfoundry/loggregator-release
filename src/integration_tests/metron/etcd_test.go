package integration_test

import (
	dopplerconfig "doppler/config"
	"doppler/dopplerservice"

	"bytes"
	"net"
	"time"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Etcd dependency", func() {
	Context("when etcd is down", func() {
		var (
			logger      *gosteno.Logger
			fakeDoppler *FakeDoppler

			stopTheWorld chan struct{}
			stopAnnounce chan chan bool
		)

		BeforeEach(func() {
			etcdRunner.Stop()

			logger = gosteno.NewLogger("test")
			dopplerConfig := &dopplerconfig.Config{
				Index:   0,
				JobName: "job",
				Zone:    "z9",
				DropsondeIncomingMessagesPort: uint32(metronRunner.DropsondePort),
				TLSListenerConfig:             dopplerconfig.TLSListenerConfig{Port: uint32(port + 5)},
			}
			stopTheWorld = make(chan struct{})

			conn := eventuallyListensForUDP(metronRunner.DropsondeAddress())
			fakeDoppler = &FakeDoppler{
				packetConn:   conn,
				stopTheWorld: stopTheWorld,
			}

			stopAnnounce = dopplerservice.AnnounceLegacy("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
		})

		AfterEach(func() {
			close(stopTheWorld)
			close(stopAnnounce)

			fakeDoppler.Close()
			metronRunner.Stop()
		})

		It("starts", func() {
			metronRunner.Protocol = "tls"
			metronRunner.Start()

			Consistently(metronRunner.Process.Wait).ShouldNot(Receive())
			Expect(metronRunner.Runner.Buffer()).To(gbytes.Say("Failed to connect to etcd"))
			Expect(metronRunner.Runner.Buffer()).To(gbytes.Say("metron started"))

			By("starting etcd")

			etcdRunner.Start()

			originalMessage := basicValueMessage()
			expectedMessage := sign(originalMessage)

			receivedByDoppler := fakeDoppler.ReadIncomingMessages(expectedMessage.signature)

			metronConn, _ := net.Dial("udp4", metronRunner.MetronAddress())
			metronInput := &MetronInput{
				metronConn:   metronConn,
				stopTheWorld: stopTheWorld,
			}
			go metronInput.WriteToMetron(originalMessage)

			Eventually(func() bool {
				select {
				case msg := <-receivedByDoppler:
					return bytes.Equal(msg.signature, expectedMessage.signature) && bytes.Equal(msg.message, expectedMessage.message)
				default:
				}
				return false
			}, 5).Should(BeTrue())
		})
	})
})
