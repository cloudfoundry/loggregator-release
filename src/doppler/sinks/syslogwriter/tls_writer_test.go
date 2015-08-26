package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	"net"
	"net/url"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("TLSWriter", func() {
	var dialer *net.Dialer

	BeforeEach(func() {
		dialer = &net.Dialer{
			Timeout: 500 * time.Millisecond,
		}
	})

	Describe("New", func() {
		It("returns an error for syslog scheme", func() {
			outputURL, _ := url.Parse("syslog://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", false, dialer)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for https scheme", func() {
			outputURL, _ := url.Parse("https://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", false, dialer)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the provided dialer is nil", func() {
			outputURL, _ := url.Parse("syslog-tls://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", false, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot construct a writer with a nil dialer"))
		})
	})

	Describe("Write", func() {
		const standardOutPriority = 14
		var syslogServerSession *gexec.Session
		var syslogWriter syslogwriter.Writer
		var skipCertVerify bool

		BeforeEach(func() {
			skipCertVerify = true
		})

		JustBeforeEach(func(done Done) {
			var err error
			syslogServerSession = startEncryptedTCPServer("127.0.0.1:9998")
			outputURL, _ := url.Parse("syslog-tls://127.0.0.1:9998")
			syslogWriter, err = syslogwriter.NewTlsWriter(outputURL, "appId", skipCertVerify, dialer)
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

		Context("when skipCertVerify is false", func() {
			BeforeEach(func() {
				skipCertVerify = false
			})

			It("rejects self-signed certs", func(done Done) {
				err := syslogWriter.Connect()
				Expect(err).To(HaveOccurred())

				close(done)
			}, 5)
		})

	})
})

func startEncryptedTCPServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "fixtures/key.crt", "-key", "fixtures/key.key")
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(drainSession.Err, 10).Should(gbytes.Say("Startup: tcp echo server listening"))

	return drainSession
}
