package doppler_test

import (
	"fmt"
	"net"

	"github.com/nu7hatch/gouuid"

	. "integration_tests/doppler/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Syslog Drain Binding", func() {

	var (
		appID           string
		inputConnection net.Conn
		syslogPort      = 6666
	)

	BeforeEach(func() {
		guid, _ := uuid.NewV4()
		appID = guid.String()
		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
	})

	AfterEach(func() {
		inputConnection.Close()
	})

	It("handles invalid schemas", func(done Done) {
		receivedChan := make(chan []byte, 1)
		ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		defer ws.Close()

		syslogDrainURL := fmt.Sprintf("syslog-invalid://%s:%d", localIPAddress, syslogPort)
		key := DrainKey(appID, syslogDrainURL)
		AddETCDNode(etcdAdapter, key, syslogDrainURL)

		receivedMessageBytes := []byte{}
		Eventually(receivedChan, 20).Should(Receive(&receivedMessageBytes))
		receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(ContainSubstring("Err: Invalid scheme type"))
		close(done)
	}, 30)

	It("handles URLs that don't resolve", func(done Done) {
		receivedChan := make(chan []byte, 1)
		ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		defer ws.Close()

		badURLs := []string{"syslog://garbage", "syslog-tls://garbage", "https://garbage"}

		for _, badURL := range badURLs {
			key := DrainKey(appID, badURL)
			AddETCDNode(etcdAdapter, key, badURL)

			receivedMessageBytes := []byte{}
			Eventually(receivedChan, 20).Should(Receive(&receivedMessageBytes))
			receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(ContainSubstring("Err: Resolving host failed"))
			Expect(string(receivedMessage.GetMessage())).To(ContainSubstring(badURL))
		}
		close(done)
	}, 30)

	Context("when connecting over TCP", func() {
		var (
			syslogDrainAddress = fmt.Sprintf("%s:%d", localIPAddress, syslogPort)
			drainSession       *gexec.Session
		)

		Context("when forwarding to an unencrypted syslog:// endpoint", func() {
			BeforeEach(func() {
				drainSession = StartUnencryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)
			})

			AfterEach(func() {
				drainSession.Kill().Wait()
			})

			It("forwards log messages to a syslog", func(done Done) {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := DrainKey(appID, syslogDrainURL)
				AddETCDNode(etcdAdapter, key, syslogDrainURL)

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))

				close(done)
			}, 100)

			It("reconnects to a reappearing syslog server after an unexpected close", func(done Done) {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := DrainKey(appID, syslogDrainURL)
				AddETCDNode(etcdAdapter, key, syslogDrainURL)

				drainSession.Kill().Wait()

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "http-message", inputConnection)
					return dopplerSession.Out
				}, 10, 1).Should(gbytes.Say(`syslog://:6666: Error when dialing out. Backing off`))

				drainSession = StartUnencryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))

				close(done)
			}, 115)
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			BeforeEach(func() {
				drainSession = StartEncryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)
			})

			AfterEach(func() {
				drainSession.Kill().Wait()
			})

			It("forwards log messages to a syslog-tls", func(done Done) {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := DrainKey(appID, syslogDrainURL)
				AddETCDNode(etcdAdapter, key, syslogDrainURL)

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))
				close(done)
			}, 100)

			It("reconnects to a reappearing tls server", func(done Done) {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := DrainKey(appID, syslogDrainURL)
				AddETCDNode(etcdAdapter, key, syslogDrainURL)

				drainSession.Kill().Wait()

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "message", inputConnection)
					return dopplerSession.Out
				}, 10, 1).Should(gbytes.Say(`syslog-tls://:6666: Error when dialing out. Backing off`))

				drainSession = StartEncryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)

				Eventually(func() *gbytes.Buffer {
					SendAppLog(appID, "syslogtls-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslogtls-message"))

				close(done)
			}, 115)
		})

	})

	Context("when forwarding to an https:// endpoint", func() {
		var (
			serverSession *gexec.Session
		)

		BeforeEach(func() {
			serverSession = StartHTTPSServer(pathToHTTPEchoServer)
			httpsURL := fmt.Sprintf("https://%s:1234/syslog/", localIPAddress)
			key := DrainKey(appID, httpsURL)
			AddETCDNode(etcdAdapter, key, httpsURL)
		})

		AfterEach(func() {
			serverSession.Kill().Wait()
		})

		It("forwards log messages to an https endpoint", func(done Done) {
			Eventually(func() *gbytes.Buffer {
				SendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 90, 1).Should(gbytes.Say(`http-message`))
			close(done)
		}, 100)

		It("reconnects a reappearing https server", func(done Done) {
			serverSession.Kill().Wait()

			SendAppLog(appID, "http-message", inputConnection)
			Eventually(dopplerSession.Out).Should(gbytes.Say(`1234/syslog.*backoff`))

			serverSession = StartHTTPSServer(pathToHTTPEchoServer)

			Eventually(func() *gbytes.Buffer {
				SendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 90, 1).Should(gbytes.Say(`http-message`))

			close(done)
		}, 100)
	})

	Context("when bound to a blacklisted drain", func() {
		It("does not forward to a TCP listener", func() {
			syslogDrainAddress := fmt.Sprintf("localhost:%d", syslogPort)
			drainSession := StartUnencryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)

			syslogDrainURL := "syslog://" + syslogDrainAddress
			key := DrainKey(appID, syslogDrainURL)
			AddETCDNode(etcdAdapter, key, syslogDrainURL)

			Consistently(func() *gbytes.Buffer {
				SendAppLog(appID, "syslog-message", inputConnection)
				return drainSession.Out
			}, 5, 1).ShouldNot(gbytes.Say("syslog-message"))

			Eventually(dopplerSession, 5).Should(gbytes.Say(`Invalid syslog drain URL \(syslog://localhost:6666\).*Err: Syslog Drain URL is blacklisted`))

			drainSession.Kill().Wait()
		})

		It("does not forward to a TCP+TLS listener", func() {
			syslogDrainAddress := fmt.Sprintf("localhost:%d", syslogPort)
			drainSession := StartEncryptedTCPServer(pathToTCPEchoServer, syslogDrainAddress)

			syslogDrainURL := "syslog-tls://" + syslogDrainAddress
			key := DrainKey(appID, syslogDrainURL)
			AddETCDNode(etcdAdapter, key, syslogDrainURL)

			Consistently(func() *gbytes.Buffer {
				SendAppLog(appID, "syslog-message", inputConnection)
				return drainSession.Out
			}, 5, 1).ShouldNot(gbytes.Say("syslog-message"))

			Eventually(dopplerSession, 5).Should(gbytes.Say(`Invalid syslog drain URL \(syslog-tls://localhost:6666\).*Err: Syslog Drain URL is blacklisted`))

			drainSession.Kill().Wait()
		})

		It("does not forward to an HTTPS listener", func() {
			serverSession := StartHTTPSServer(pathToHTTPEchoServer)
			httpsURL := "https://localhost:1234/syslog/"
			key := DrainKey(appID, httpsURL)
			AddETCDNode(etcdAdapter, key, httpsURL)

			Consistently(func() *gbytes.Buffer {
				SendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 5, 1).ShouldNot(gbytes.Say(`http-message`))

			Eventually(dopplerSession).Should(gbytes.Say(`Invalid syslog drain URL \(https://localhost:1234/syslog/\).*Err: Syslog Drain URL is blacklisted`))

			serverSession.Kill().Wait()
		})
	})
})
