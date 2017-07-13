package syslogwriter_test

import (
	"crypto/tls"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslogwriter"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("TLSWriter", func() {
	var dialer *net.Dialer
	var ioTimeout time.Duration

	BeforeEach(func() {
		ioTimeout = 0
		dialer = &net.Dialer{
			Timeout: 500 * time.Millisecond,
		}
	})

	Describe("New", func() {
		It("returns an error for syslog scheme", func() {
			outputURL, _ := url.Parse("syslog://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", "hostname", false, dialer, ioTimeout)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for https scheme", func() {
			outputURL, _ := url.Parse("https://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", "hostname", false, dialer, ioTimeout)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the provided dialer is nil", func() {
			outputURL, _ := url.Parse("syslog-tls://localhost")
			_, err := syslogwriter.NewTlsWriter(outputURL, "appId", "hostname", false, nil, ioTimeout)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot construct a writer with a nil dialer"))
		})

		It("requires TLS Version 1.2", func() {
			outputURL, _ := url.Parse("syslog-tls://localhost")
			w, err := syslogwriter.NewTlsWriter(outputURL, "appId", "hostname", false, dialer, ioTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(w.TlsConfig.MinVersion).To(BeEquivalentTo(tls.VersionTLS12))
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

		JustBeforeEach(func() {
			dialer = &net.Dialer{
				Timeout: time.Second,
			}

			var err error
			port := 9900 + config.GinkgoConfig.ParallelNode
			address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			syslogServerSession = startEncryptedTCPServer(address)
			outputURL := &url.URL{Scheme: "syslog-tls", Host: address}
			syslogWriter, err = syslogwriter.NewTlsWriter(outputURL, "appId", "hostname", skipCertVerify, dialer, ioTimeout)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			syslogServerSession.Terminate().Wait()
			syslogWriter.Close()
		})

		XIt("connects and writes", func() {
			ts := time.Now().UnixNano()
			Eventually(syslogWriter.Connect, 5, 1).Should(Succeed())

			_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "test", "", ts)
			Expect(err).ToNot(HaveOccurred())

			Eventually(syslogServerSession, 3).Should(gbytes.Say("just a test"))
		})

		Context("when an i/o timeout is set", func() {
			BeforeEach(func() {
				// cause an immediate write timeout
				ioTimeout = -1 * time.Second
			})

			It("returns a timeout error", func() {
				Eventually(func() error {
					err := syslogWriter.Connect()
					return err
				}, 5, 1).ShouldNot(HaveOccurred())

				_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "test", "", time.Now().UnixNano())
				Expect(err).To(HaveOccurred())
				netErr := err.(*net.OpError)
				Expect(netErr.Timeout()).To(BeTrue())
			})
		})

		Context("won't write to invalid syslog drains", func() {
			It("returns an error when unable to send the log message", func() {
				syslogServerSession.Kill().Wait()

				Eventually(func() error {
					_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
					return err
				}, 5).Should(HaveOccurred())
			}, 10)

			It("returns an error if not connected", func() {
				syslogWriter.Close()
				_, err := syslogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				Expect(err).To(HaveOccurred())
			}, 5)
		})

		Context("when skipCertVerify is false", func() {
			BeforeEach(func() {
				skipCertVerify = false
			})

			It("rejects self-signed certs", func() {
				err := syslogWriter.Connect()
				Expect(err).To(HaveOccurred())
			}, 5)
		})
	})
})

func startEncryptedTCPServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(
		pathToTCPEchoServer,
		"-address",
		syslogDrainAddress,
		"-ssl",
		"-cert",
		fixture("key.crt"),
		"-key",
		fixture("key.key"),
	)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(drainSession.Err, 10).Should(gbytes.Say("tcp echo server listening"))
	Eventually(func() error {
		c, err := net.Dial("tcp", syslogDrainAddress)
		if err == nil {
			c.Close()
		}
		return err
	}).ShouldNot(HaveOccurred())

	return drainSession
}
