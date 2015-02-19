package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"net/url"
	"sync"
	"time"
)

var _ = Describe("SyslogWriter", func() {

	var dataChan <-chan []byte
	var serverStoppedChan <-chan bool
	var shutdownChan chan bool
	var sysLogWriter syslogwriter.Writer
	standardOutPriority := 14

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
		It("sends messages in the proper format", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte("just a test"), "App", "2", time.Now().UnixNano())

			data := <-dataChan
			Expect(string(data)).To(MatchRegexp(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
			close(done)
		})

		It("strips null termination char from message", func(done Done) {
			sysLogWriter.Write(standardOutPriority, []byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

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

	var listenerStopped sync.WaitGroup
	listenerStopped.Add(1)

	go func() {
		<-shutdownChan
		listener.Close()
		listenerStopped.Wait()
		close(doneChan)
	}()

	go func() {
		defer listenerStopped.Done()
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		readCount, err := conn.Read(buffer)
		buffer2 := make([]byte, readCount)
		copy(buffer2, buffer[:readCount])
		dataChan <- buffer2
		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return dataChan, doneChan
}
