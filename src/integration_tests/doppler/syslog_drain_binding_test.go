package doppler_test

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/nu7hatch/gouuid"
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
		ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		defer ws.Close()

		syslogDrainURL := fmt.Sprintf("syslog-invalid://%s:%d", localIPAddress, syslogPort)
		key := drainKey(appID, syslogDrainURL)
		addETCDNode(key, syslogDrainURL)

		receivedMessageBytes := []byte{}
		Eventually(receivedChan, 20).Should(Receive(&receivedMessageBytes))
		receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(ContainSubstring("Err: Invalid scheme type"))
		close(done)
	}, 30)

	It("handles URLs that don't resolve", func(done Done) {
		receivedChan := make(chan []byte, 1)
		ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		defer ws.Close()

		badURLs := []string{"syslog://garbage", "syslog-tls://garbage", "https://garbage"}

		for _, badURL := range badURLs {
			key := drainKey(appID, badURL)
			addETCDNode(key, badURL)

			receivedMessageBytes := []byte{}
			Eventually(receivedChan, 20).Should(Receive(&receivedMessageBytes))
			receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

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
				drainSession = startUnencryptedTCPServer(syslogDrainAddress)
			})

			AfterEach(func() {
				drainSession.Kill()
			})

			It("forwards log messages to a syslog", func(done Done) {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))

				close(done)
			}, 100)

			It("reconnects to a reappearing syslog server after an unexpected close", func(done Done) {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				drainSession.Kill()

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "http-message", inputConnection)
					return dopplerSession.Out
				}, 10, 1).Should(gbytes.Say(`syslog://:6666: Error when dialing out. Backing off`))

				drainSession = startUnencryptedTCPServer(syslogDrainAddress)

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))

				close(done)
			}, 115)
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			BeforeEach(func() {
				drainSession = startEncryptedTCPServer(syslogDrainAddress)
			})

			AfterEach(func() {
				drainSession.Kill()
			})

			It("forwards log messages to a syslog-tls", func(done Done) {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "syslog-message", inputConnection)
					return drainSession.Out
				}, 90, 1).Should(gbytes.Say("syslog-message"))
				close(done)
			}, 100)

			It("reconnects to a reappearing tls server", func(done Done) {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				drainSession.Kill()

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "message", inputConnection)
					return dopplerSession.Out
				}, 10, 1).Should(gbytes.Say(`syslog-tls://:6666: Error when dialing out. Backing off`))

				drainSession = startEncryptedTCPServer(syslogDrainAddress)

				Eventually(func() *gbytes.Buffer {
					sendAppLog(appID, "syslogtls-message", inputConnection)
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
			serverSession = startHTTPSServer()
			httpsURL := fmt.Sprintf("https://%s:1234/syslog/", localIPAddress)
			key := drainKey(appID, httpsURL)
			addETCDNode(key, httpsURL)
		})

		AfterEach(func() {
			serverSession.Kill()
		})

		It("forwards log messages to an https endpoint", func(done Done) {
			Eventually(func() *gbytes.Buffer {
				sendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 90, 1).Should(gbytes.Say(`http-message`))
			close(done)
		}, 100)

		It("reconnects a reappearing https server", func(done Done) {
			serverSession.Kill()
			<-time.After(100 * time.Millisecond)

			sendAppLog(appID, "http-message", inputConnection)
			Eventually(dopplerSession.Out).Should(gbytes.Say(`1234/syslog.*backoff`))

			serverSession = startHTTPSServer()

			Eventually(func() *gbytes.Buffer {
				sendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 90, 1).Should(gbytes.Say(`http-message`))

			close(done)
		}, 100)
	})

	Context("when bound to a blacklisted drain", func() {
		It("does not forward to a TCP listener", func() {
			syslogDrainAddress := fmt.Sprintf("localhost:%d", syslogPort)
			drainSession := startUnencryptedTCPServer(syslogDrainAddress)

			syslogDrainURL := "syslog://" + syslogDrainAddress
			key := drainKey(appID, syslogDrainURL)
			addETCDNode(key, syslogDrainURL)

			Consistently(func() *gbytes.Buffer {
				sendAppLog(appID, "syslog-message", inputConnection)
				return drainSession.Out
			}, 5, 1).ShouldNot(gbytes.Say("syslog-message"))

			Eventually(dopplerSession, 5).Should(gbytes.Say(`Invalid syslog drain URL \(syslog://localhost:6666\).*Err: Syslog Drain URL is blacklisted`))

			drainSession.Kill()
		})

		It("does not forward to a TCP+TLS listener", func() {
			syslogDrainAddress := fmt.Sprintf("localhost:%d", syslogPort)
			drainSession := startEncryptedTCPServer(syslogDrainAddress)

			syslogDrainURL := "syslog-tls://" + syslogDrainAddress
			key := drainKey(appID, syslogDrainURL)
			addETCDNode(key, syslogDrainURL)

			Consistently(func() *gbytes.Buffer {
				sendAppLog(appID, "syslog-message", inputConnection)
				return drainSession.Out
			}, 5, 1).ShouldNot(gbytes.Say("syslog-message"))

			Eventually(dopplerSession, 5).Should(gbytes.Say(`Invalid syslog drain URL \(syslog-tls://localhost:6666\).*Err: Syslog Drain URL is blacklisted`))

			drainSession.Kill()
		})

		It("does not forward to an HTTPS listener", func() {
			serverSession := startHTTPSServer()
			httpsURL := "https://localhost:1234/syslog/"
			key := drainKey(appID, httpsURL)
			addETCDNode(key, httpsURL)

			Consistently(func() *gbytes.Buffer {
				sendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 5, 1).ShouldNot(gbytes.Say(`http-message`))

			Eventually(dopplerSession).Should(gbytes.Say(`Invalid syslog drain URL \(https://localhost:1234/syslog/\).*Err: Syslog Drain URL is blacklisted`))

			serverSession.Kill()
		})
	})
})

func startHTTPSServer() *gexec.Session {
	command := exec.Command(pathToHTTPEchoServer, "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

func startUnencryptedTCPServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}

func startEncryptedTCPServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}
