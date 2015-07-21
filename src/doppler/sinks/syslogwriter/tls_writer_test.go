package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	"net/url"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("TLSWriter", func() {
	var syslogServerSession *gexec.Session
	var syslogWriter syslogwriter.Writer
	var standardOutPriority int = 14

	Context("writes and connects to syslog tls drains", func() {
		BeforeEach(func(done Done) {
			var err error
			syslogServerSession = startEncryptedTCPServer("127.0.0.1:9998")
			outputURL, _ := url.Parse("syslog-tls://127.0.0.1:9998")
			syslogWriter, err = syslogwriter.NewTlsWriter(outputURL, "appId", true)
			Expect(err).ToNot(HaveOccurred())
			close(done)
		}, 5)

		AfterEach(func() {
			syslogServerSession.Kill().Wait()
			syslogWriter.Close()
		})

		It("connects and writes", func(done Done) {
			ts := time.Now().UnixNano()
			Eventually(func() error {
				err := syslogWriter.Connect()
				return err
			}, 5, 1).ShouldNot(HaveOccurred())

			_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "test", "", ts)
			Expect(err).ToNot(HaveOccurred())

			Eventually(syslogServerSession, 3).Should(gbytes.Say("just a test"))
			close(done)
		}, 10)

		Context("won't write to invalid syslog drains", func() {
			It("returns an error when unable to send the log message", func(done Done) {
				syslogServerSession.Kill().Wait()

				Eventually(func() error {
					_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
					return err
				}, 5).Should(HaveOccurred())
				close(done)
			}, 10)

			It("returns an error if not connected", func(done Done) {
				syslogWriter.Close()
				_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				Expect(err).To(HaveOccurred())
				close(done)
			}, 5)
		})
	})

	It("rejects self-signed certs when skipCertVerify is false", func(done Done) {
		syslogServerSession = startEncryptedTCPServer("127.0.0.1:9998")
		outputURL, _ := url.Parse("syslog-tls://localhost:9998")

		syslogWriter, _ = syslogwriter.NewTlsWriter(outputURL, "appId", false)
		err := syslogWriter.Connect()
		Expect(err).To(HaveOccurred())

		syslogServerSession.Kill().Wait()
		syslogWriter.Close()
		close(done)
	}, 5)

	It("returns an error for syslog scheme", func() {
		outputURL, _ := url.Parse("syslog://localhost")
		_, err := syslogwriter.NewTlsWriter(outputURL, "appId", false)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputURL, _ := url.Parse("https://localhost")
		_, err := syslogwriter.NewTlsWriter(outputURL, "appId", false)
		Expect(err).To(HaveOccurred())
	})
})

func startEncryptedTCPServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(drainSession.Err, 10).Should(gbytes.Say("Startup: tcp echo server listening"))

	return drainSession
}
