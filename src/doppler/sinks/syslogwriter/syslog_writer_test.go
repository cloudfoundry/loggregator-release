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

var _ = Describe("SyslogWriter", func() {

	var sysLogWriter syslogwriter.Writer
	standardOutPriority := 14

	var syslogServerSession *gexec.Session
	BeforeEach(func(done Done) {
		outputURL, _ := url.Parse("syslog://127.0.0.1:9999")
		syslogServerSession = startSyslogServer("127.0.0.1:9999")
		sysLogWriter, _ = syslogwriter.NewSyslogWriter(outputURL, "appId")

		Eventually(func() error {
			err := sysLogWriter.Connect()
			return err
		}, 5).ShouldNot(HaveOccurred())

		close(done)
	}, 10)

	AfterEach(func() {
		sysLogWriter.Close()
		syslogServerSession.Kill()
	})

	Context("Message Format", func() {
		It("sends messages in the proper format", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())

			Eventually(syslogServerSession, 5).Should(gbytes.Say(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
			close(done)
		}, 10)

		It("strips null termination char from message", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

			Expect(syslogServerSession).ToNot(gbytes.Say("\000"))

			close(done)
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

	It("returns an error for syslog-tls scheme", func() {
		outputURL, _ := url.Parse("syslog-tls://localhost")
		_, err := syslogwriter.NewSyslogWriter(outputURL, "appId")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputURL, _ := url.Parse("https://localhost")
		_, err := syslogwriter.NewSyslogWriter(outputURL, "appId")
		Expect(err).To(HaveOccurred())
	})

	Context("Message Format", func() {
		It("sends messages in the proper format", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())

			Eventually(syslogServerSession, 5).Should(gbytes.Say(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
			close(done)
		}, 10)

		It("strips null termination char from message", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

			Expect(syslogServerSession).ToNot(gbytes.Say("\000"))

			close(done)
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
})

func startSyslogServer(syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(1000 * time.Millisecond) // give time for server to come up
	return drainSession
}
