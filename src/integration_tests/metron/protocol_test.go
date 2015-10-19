package integration_test

import (
	"bytes"
	"net"
	"time"

	dopplerconfig "doppler/config"
	"doppler/dopplerservice"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Protocol", func() {
	var (
		preferredProtocol string
		logger            *gosteno.Logger
		dopplerConfig     *dopplerconfig.Config

		stopTheWorld chan struct{}
		stopAnnounce chan chan bool
	)

	BeforeEach(func() {
		logger = gosteno.NewLogger("test")
		dopplerConfig = &dopplerconfig.Config{
			Index:   0,
			JobName: "job",
			Zone:    "z9",
			DropsondeIncomingMessagesPort: uint32(dropsondePort),
			TLSListenerConfig:             dopplerconfig.TLSListenerConfig{Port: uint32(port + 5)},
		}
		stopTheWorld = make(chan struct{})
	})

	JustBeforeEach(func() {
		startMetron(preferredProtocol)
	})

	AfterEach(func() {
		close(stopTheWorld)
		close(stopAnnounce)

		stopMetron()
	})

	Context("Doppler over UDP", func() {
		var fakeDoppler *FakeDoppler

		BeforeEach(func() {
			conn := eventuallyListensForUDP(dropsondeAddress())
			fakeDoppler = &FakeDoppler{
				packetConn:   conn,
				stopTheWorld: stopTheWorld,
			}

		})

		AfterEach(func() {
			fakeDoppler.Close()
		})

		itReceives := func(additional ...func()) {
			It("forwards hmac signed messages to a healthy doppler server", func() {
				originalMessage := basicValueMessage()
				expectedMessage := sign(originalMessage)

				receivedByDoppler := fakeDoppler.ReadIncomingMessages(expectedMessage.signature)

				metronConn, _ := net.Dial("udp4", metronAddress())
				metronInput := &MetronInput{
					metronConn:   metronConn,
					stopTheWorld: stopTheWorld,
				}
				go metronInput.WriteToMetron(originalMessage)

				Eventually(func() bool {
					var msg signedMessage
					Eventually(receivedByDoppler).Should(Receive(&msg))
					return bytes.Equal(msg.signature, expectedMessage.signature) && bytes.Equal(msg.message, expectedMessage.message)
				}).Should(BeTrue())

				for _, f := range additional {
					f()
				}
			}, 2)
		}

		Context("Metron prefers UDP", func() {
			BeforeEach(func() {
				preferredProtocol = "udp"
			})

			Context("Legacy Doppler", func() {
				BeforeEach(func() {
					stopAnnounce = dopplerservice.AnnounceLegacy("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives()
			})

			Context("Doppler advertises UDP", func() {
				BeforeEach(func() {
					stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives()
			})

			Context("Doppler advertises TLS", func() {
				BeforeEach(func() {
					dopplerConfig.EnableTLSTransport = true
					stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives()
			})
		})

		Context("Metron prefers TLS", func() {
			BeforeEach(func() {
				preferredProtocol = "tls"
			})

			Context("Legacy Doppler", func() {
				BeforeEach(func() {
					stopAnnounce = dopplerservice.AnnounceLegacy("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives(func() {
					Expect(metronRunner.Buffer()).To(Say("Doppler advertising UDP only"))
				})
			})

			Context("Doppler over UDP", func() {
				BeforeEach(func() {
					stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives(func() {
					Expect(metronRunner.Buffer()).To(Say("Doppler advertising UDP only"))
				})
			})
		})
	})

	Context("Doppler over TLS", func() {
		BeforeEach(func() {
			dopplerConfig.EnableTLSTransport = true
			stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
		})

		Context("Metron prefers TLS", func() {
			BeforeEach(func() {
				preferredProtocol = "tls"
			})

			It("logs a sent message", func() {
				metronConn, _ := net.Dial("udp4", metronAddress())
				metronInput := &MetronInput{
					metronConn:   metronConn,
					stopTheWorld: stopTheWorld,
				}
				originalMessage := basicValueMessage()
				go metronInput.WriteToMetron(originalMessage)

				Eventually(metronRunner.Buffer()).Should(Say("Sending over TLS"))
			})
		})
	})
})
