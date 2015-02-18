package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"net/url"
	"time"
)

var _ = Describe("SyslogWriter", func() {

	var dataChan <-chan []byte
	var serverStoppedChan <-chan bool
	var shutdownChan chan bool
	var sysLogWriter syslogwriter.SyslogWriter

	BeforeEach(func() {
		shutdownChan = make(chan bool)
		dataChan, serverStoppedChan = startSyslogServer(shutdownChan)
		outputUrl, _ := url.Parse("syslog://localhost:9999")
		sysLogWriter, _ = syslogwriter.NewSyslogWriter(outputUrl, "appId")
		sysLogWriter.Connect()
	})

	AfterEach(func() {
		close(shutdownChan)
		sysLogWriter.Close()
		<-serverStoppedChan
	})

	It("returns an error for syslog-tls scheme", func() {
		outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
		_, err := syslogwriter.NewSyslogWriter(outputUrl, "appId")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputUrl, _ := url.Parse("https://localhost:9999")
		_, err := syslogwriter.NewSyslogWriter(outputUrl, "appId")
		Expect(err).To(HaveOccurred())
	})

	Context("Message Format", func() {
		It("sends messages from stdout with INFO priority", func(done Done) {
			sysLogWriter.WriteStdout([]byte("just a test"), "test", "", time.Now().UnixNano())

			data := <-dataChan
			Expect(string(data)).To(MatchRegexp(`\d <14>\d `))
			close(done)
		})

		It("sends messages from stderr with ERROR priority", func(done Done) {
			sysLogWriter.WriteStderr([]byte("just a test"), "test", "", time.Now().UnixNano())

			data := <-dataChan
			Expect(string(data)).To(MatchRegexp(`\d <11>\d `))
			close(done)
		})

		It("sends messages in the proper format", func(done Done) {
			sysLogWriter.WriteStdout([]byte("just a test"), "App", "2", time.Now().UnixNano())

			data := <-dataChan
			Expect(string(data)).To(MatchRegexp(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
			close(done)
		})

		It("strips null termination char from message", func(done Done) {
			sysLogWriter.WriteStdout([]byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

			data := <-dataChan
			Expect(string(data)).ToNot(MatchRegexp("\000"))
			close(done)
		})
	})
})

func startSyslogServer(shutdownChan <-chan bool) (<-chan []byte, <-chan bool) {
	dataChan := make(chan []byte)
	doneChan := make(chan bool)
	listener, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		panic(err)
	}

	go func() {
		<-shutdownChan
		listener.Close()
		close(doneChan)
	}()

	go func() {
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		readCount, err := conn.Read(buffer)
		buffer2 := make([]byte, readCount)
		copy(buffer2, buffer[:readCount])
		dataChan <- buffer2
		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return dataChan, doneChan
}
