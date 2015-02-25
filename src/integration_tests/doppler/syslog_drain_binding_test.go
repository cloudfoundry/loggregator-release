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
		Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
		receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(ContainSubstring("Err: Invalid scheme type"))
		close(done)
	}, 10)

	It("handles URLs that don't resolve", func(done Done) {
		receivedChan := make(chan []byte, 1)
		ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		defer ws.Close()

		badURLs := []string{"syslog://garbage", "syslog-tls://garbage", "https://garbage"}

		for _, badURL := range badURLs {
			key := drainKey(appID, badURL)
			addETCDNode(key, badURL)

			receivedMessageBytes := []byte{}
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
			receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(ContainSubstring("Err: Resolving host failed"))
			Expect(string(receivedMessage.GetMessage())).To(ContainSubstring(badURL))
		}
		close(done)
	}, 10)

	Context("when connecting over TCP", func() {
		var (
			syslogDrainAddress = fmt.Sprintf("%s:%d", localIPAddress, syslogPort)
			drainSession       *gexec.Session
		)

		AfterEach(func() {
			drainSession.Kill()
		})

		Context("when forwarding to an unencrypted syslog:// endpoint", func() {
			BeforeEach(func() {
				var err error
				command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
				drainSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
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
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			BeforeEach(func() {
				var err error
				command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
				drainSession, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
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
		})

	})

	Context("when forwarding to an https:// endpoint", func() {
		var (
			serverSession *gexec.Session
		)

		BeforeEach(func() {
			serverSession = startHTTPSServer()
			httpsURL := "https://localhost:1234/syslog/"
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
			}, 10, 1).Should(gbytes.Say(`http-message`))
			close(done)
		}, 15)

		It("reconnects a reappearing https server", func(done Done) {
			serverSession.Kill()
			<-time.After(100 * time.Millisecond)

			sendAppLog(appID, "http-message", inputConnection)
			Eventually(dopplerSession.Out).Should(gbytes.Say(`1234/syslog.*backoff`))

			serverSession = startHTTPSServer()

			Eventually(func() *gbytes.Buffer {
				sendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 10, 1).Should(gbytes.Say(`http-message`))

			close(done)
		}, 15)

	})
})

func startHTTPSServer() *gexec.Session {
	command := exec.Command(pathToHTTPEchoServer, "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}
