package integration_test

import (
	"bytes"
	"fmt"
	"metron/config"
	"net"
	"time"

	dopplerconfig "doppler/config"
	"doppler/dopplerservice"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Protocol", func() {
	var (
		protocols     []config.Protocol
		logger        *gosteno.Logger
		dopplerConfig *dopplerconfig.Config

		stopTheWorld chan struct{}
		stopAnnounce chan chan bool
	)

	BeforeEach(func() {
		protocols = nil
		logger = gosteno.NewLogger("test")
		dopplerConfig = &dopplerconfig.Config{
			Index:           0,
			JobName:         "job",
			Zone:            "z9",
			IncomingUDPPort: uint32(metronRunner.DropsondePort),
			TLSListenerConfig: dopplerconfig.TLSListenerConfig{
				Port:     uint32(port + 5),
				CertFile: "../fixtures/server.crt",
				KeyFile:  "../fixtures/server.key",
				CAFile:   "../fixtures/loggregator-ca.crt",
			},
		}
		stopTheWorld = make(chan struct{})
		stopAnnounce = nil
	})

	JustBeforeEach(func() {
		metronRunner.Protocols = protocols
		metronRunner.Start()
	})

	AfterEach(func() {
		close(stopTheWorld)
		close(stopAnnounce)

		metronRunner.Stop()
	})

	Describe("Metron panics", func() {
		Context("with Metron configured to requires TLS, and TLS disabled on Doppler", func() {
			BeforeEach(func() {
				protocols = []config.Protocol{"tls"}
				dopplerConfig.EnableTLSTransport = false
			})

			Context("with Doppler advertising only on legacy endpoint", func() {
				It("panics", func() {
					stopAnnounce = dopplerservice.AnnounceLegacy("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
					Eventually(metronRunner.Process.Wait()).Should(Receive())
				})
			})

			Context("with Doppler advertising UDP on meta endpoint", func() {
				It("panics", func() {
					stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
					Eventually(metronRunner.Process.Wait()).Should(Receive())
				})
			})
		})
	})

	Describe("Metron doesn't panic", func() {
		Context("with Doppler advertising over UDP and TCP", func() {
			var fakeDoppler *FakeDoppler

			BeforeEach(func() {
				conn := eventuallyListensForUDP(metronRunner.DropsondeAddress())
				fakeDoppler = &FakeDoppler{
					packetConn:   conn,
					stopTheWorld: stopTheWorld,
				}
			})

			AfterEach(func() {
				fakeDoppler.Close()
			})

			itReceives := func() {
				It("forwards hmac signed messages to a healthy doppler server", func() {
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
					}).Should(BeTrue())
				}, 2)
			}

			Context("with metron configured to communicate over UDP", func() {
				BeforeEach(func() {
					protocols = []config.Protocol{"udp"}
				})

				Context("with Doppler advertising over legacy endpoint", func() {
					BeforeEach(func() {
						stopAnnounce = dopplerservice.AnnounceLegacy("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
					})

					itReceives()
				})

				Context("with Doppler advertising over META endpoint", func() {
					Context("with Doppler advertising UDP", func() {
						BeforeEach(func() {
							stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
						})

						itReceives()
					})

					Context("with Doppler advertising TLS", func() {
						BeforeEach(func() {
							dopplerConfig.EnableTLSTransport = true
							stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
						})

						itReceives()
					})
				})
			})

			Context("with metron configured for to prefer TLS", func() {
				BeforeEach(func() {
					protocols = []config.Protocol{"tls", "udp"}
					stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
				})

				itReceives()
			})
		})

		Context("with Doppler advertising over TLS, UDP, and TCP", func() {
			BeforeEach(func() {
				dopplerConfig.EnableTLSTransport = true
				stopAnnounce = dopplerservice.Announce("127.0.0.1", time.Minute, dopplerConfig, etcdAdapter, logger)
			})

			Context("with Metron configured to communicate over TLS", func() {
				var (
					tlsListener net.Listener
					connChan    chan net.Conn
				)

				BeforeEach(func() {
					protocols = []config.Protocol{"tls"}
					address := fmt.Sprintf("127.0.0.1:%d", dopplerConfig.TLSListenerConfig.Port)
					tlsListener = eventuallyListensForTLS(address)

					connChan = make(chan net.Conn, 1)

					tlsListener := tlsListener
					connChan := connChan
					go func() {
						defer GinkgoRecover()
						for {
							conn, err := tlsListener.Accept()
							if err != nil {
								return
							}
							if conn != nil {
								connChan <- conn
							}
						}
					}()
				})

				AfterEach(func() {
					tlsListener.Close()
				})

				It("logs a sent message", func() {
					metronConn, _ := net.Dial("udp4", metronRunner.MetronAddress())
					metronInput := &MetronInput{
						metronConn:   metronConn,
						stopTheWorld: stopTheWorld,
					}
					originalMessage := basicValueMessage()
					go metronInput.WriteToMetron(originalMessage)

					var conn net.Conn
					Eventually(connChan).Should(Receive(&conn))
					buffer := make([]byte, 10)
					n, err := conn.Read(buffer)
					Expect(err).NotTo(HaveOccurred())
					Expect(n).To(BeNumerically(">", 0))

					conn.Close()
				})
			})
		})
	})
})
