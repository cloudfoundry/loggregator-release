package doppler_test

import (
	"fmt"
	"net"
	"os/exec"

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
		shutdownChan    chan struct{}
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

	It("handles invalid schemas", func() {
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
	})

	It("handles URLs that don't resolve", func() {
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
	})

	Context("when connecting over TCP", func() {
		var (
			syslogDrainAddress string
			dataChan           <-chan []byte
			serverStoppedChan  <-chan struct{}
		)

		BeforeEach(func() {
			syslogDrainAddress = fmt.Sprintf("%s:%d", localIPAddress, syslogPort)

			shutdownChan = make(chan struct{})
		})

		AfterEach(func() {
			close(shutdownChan)
			<-serverStoppedChan
		})

		Context("when forwarding to an unencrypted syslog:// endpoint", func() {
			BeforeEach(func() {
				dataChan, serverStoppedChan = startSyslogServer(shutdownChan, syslogDrainAddress)
			})

			It("forwards log messages to a syslog", func() {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() string {
					sendAppLog(appID, "standard syslog msg", inputConnection)
					select {
					case message := <-dataChan:
						return string(message)
					default:
						return ""
					}

				}, 90, 1).Should(ContainSubstring("standard syslog msg"))
			})
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			BeforeEach(func() {
				dataChan, serverStoppedChan = startSyslogTLSServer(shutdownChan, syslogDrainAddress)
			})

			It("forwards log messages to a syslog-tls", func() {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() string {
					sendAppLog(appID, "tls-message", inputConnection)
					select {
					case message := <-dataChan:
						return string(message)
					default:
						return ""
					}
				}, 90, 1).Should(ContainSubstring("tls-message"))
			})
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

		It("forwards log messages to an https endpoint", func() {
			Eventually(func() *gbytes.Buffer {
				sendAppLog(appID, "http-message", inputConnection)
				return serverSession.Out
			}, 10, 1).Should(gbytes.Say(`http-message`))
		})
	})
})

func startHTTPSServer() *gexec.Session {
	command := exec.Command(pathToEchoServer, "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}
