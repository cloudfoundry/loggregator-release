package syslogwriter_test

import (
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

const standardOutPriority = 14

var _ = Describe("SyslogWriter", func() {

	var sysLogWriter syslogwriter.Writer
	var dialer *net.Dialer
	var syslogServerSession *gexec.Session

	BeforeEach(func() {
		dialer = &net.Dialer{
			Timeout: 500 * time.Millisecond,
		}

		port := 9800 + config.GinkgoConfig.ParallelNode
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

		outputURL := &url.URL{Scheme: "syslog", Host: address}
		syslogServerSession = startSyslogServer(address)
		sysLogWriter, _ = syslogwriter.NewSyslogWriter(
			outputURL,
			"appId",
			"org-name.space-name.app-name.1",
			dialer,
			0,
		)

		Eventually(func() error {
			err := sysLogWriter.Connect()
			return err
		}, 5, 1).ShouldNot(HaveOccurred())
	}, 10)

	AfterEach(func() {
		sysLogWriter.Close()
		syslogServerSession.Kill().Wait()
	})

	Context("Message Format", func() {
		It("sends messages in the proper format", func() {
			sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())

			Eventually(syslogServerSession, 5).Should(gbytes.Say(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) org-name.space-name.app-name.1 appId \[APP/2\] - - just a test\n`))
		}, 10)

		It("sends messages in the proper format with source type APP/<AnyThing>", func() {
			sysLogWriter.Write(standardOutPriority, []byte("just a test"), "APP/PROC/BLAH", "2", time.Now().UnixNano())

			Eventually(syslogServerSession, 5).Should(gbytes.Say(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) org-name.space-name.app-name.1 appId \[APP/PROC/BLAH/2\] - - just a test\n`))
		}, 10)

		It("strips null termination char from message", func() {
			sysLogWriter.Write(standardOutPriority, []byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

			Expect(syslogServerSession).ToNot(gbytes.Say("\000"))
		})
	})

	Context("won't write to invalid syslog drains", func() {
		It("returns an error when unable to send the log message", func() {
			syslogServerSession.Kill().Wait()

			Eventually(func() error {
				_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				return err
			}).Should(HaveOccurred())
		})

		It("returns an error if not connected", func() {
			sysLogWriter.Close()
			_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})
	})

	It("returns an error when the provided dialer is nil", func() {
		outputURL, _ := url.Parse("syslog://localhost")
		_, err := syslogwriter.NewSyslogWriter(outputURL, "appId", "hostname", nil, 0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot construct a writer with a nil dialer"))
	})

	It("returns an error for syslog-tls scheme", func() {
		outputURL, _ := url.Parse("syslog-tls://localhost")
		_, err := syslogwriter.NewSyslogWriter(outputURL, "appId", "hostname", dialer, 0)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputURL, _ := url.Parse("https://localhost")
		_, err := syslogwriter.NewSyslogWriter(outputURL, "appId", "hostname", dialer, 0)
		Expect(err).To(HaveOccurred())
	})

	Context("won't write to invalid syslog drains", func() {
		It("returns an error when unable to send the log message", func() {
			syslogServerSession.Kill().Wait()

			Eventually(func() error {
				_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				return err
			}).Should(HaveOccurred())
		})

		It("returns an error if not connected", func() {
			sysLogWriter.Close()
			_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("network timeouts", func() {
		var listener net.Listener
		var acceptedConns chan net.Conn
		var sysLogWriter syslogwriter.Writer
		var writeTimeout time.Duration

		BeforeEach(func() {
			writeTimeout = 0
		})

		JustBeforeEach(func() {
			var err error
			listener, err = net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())

			url, err := url.Parse("syslog://" + listener.Addr().String())
			Expect(err).NotTo(HaveOccurred())

			sysLogWriter, err = syslogwriter.NewSyslogWriter(url, "appId", "hostname", dialer, writeTimeout)
			Expect(err).NotTo(HaveOccurred())

			acceptedConns = make(chan net.Conn, 1)
			go startListener(listener, acceptedConns)

			err = sysLogWriter.Connect()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			listener.Close()
			sysLogWriter.Close()
		})

		Context("when the drain is slow to consume", func() {
			BeforeEach(func() {
				// force an immediate write timeout
				writeTimeout = -1 * time.Second
			})

			It("returns an error after the write deadline expires", func() {
				_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				opErr := err.(*net.OpError)
				Expect(opErr.Timeout()).To(BeTrue())
			})
		})

		Context("when the server connection closes", func() {
			It("gets detected by watch connection", func() {
				written, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				Expect(err).NotTo(HaveOccurred())
				Expect(written).NotTo(Equal(0))

				var conn net.Conn
				Eventually(acceptedConns).Should(Receive(&conn))

				err = conn.Close()
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					_, err := sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
					return err
				}).Should(MatchError("Connection to syslog sink lost"))

				err = sysLogWriter.Connect()
				Expect(err).NotTo(HaveOccurred())

				written, err = sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())
				Expect(err).NotTo(HaveOccurred())
				Expect(written).NotTo(Equal(0))
			})
		})
	})

})

func startSyslogServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(1000 * time.Millisecond) // give time for server to come up
	return drainSession
}

func startListener(listener net.Listener, acceptedConns chan<- net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		acceptedConns <- conn
	}
}
