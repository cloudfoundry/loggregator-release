package loggingstream_test

import (
	"bufio"
	"deaagent/domain"
	"deaagent/loggingstream"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"
)

var _ = Describe("LoggingStream", func() {

	var loggingStream *loggingstream.LoggingStream
	var socketPath string

	BeforeEach(func() {
		tmpdir, _ := ioutil.TempDir("", "testing")
		task := &domain.Task{
			ApplicationId:       "1234",
			WardenJobId:         42,
			WardenContainerPath: tmpdir,
			Index:               1,
			SourceName:          "App",
		}

		value := task.Identifier()
		os.MkdirAll(value, 0777)

		socketPath = filepath.Join(task.Identifier(), "stdout.sock")
		loggingStream = loggingstream.NewLoggingStream(task, loggertesthelper.Logger(), events.LogMessage_OUT)
		loggertesthelper.TestLoggerSink.Clear()
	})

	Describe("FetchReader", func() {

		Context("when socket never opens", func() {
			It("closes return channel without sending", func() {
				readerChan := make(chan io.Reader)

				go loggingStream.FetchReader(readerChan)
				Consistently(readerChan).Should(BeEmpty())
				_, ok := <-readerChan
				Expect(ok).To(BeFalse())
			})
		})

		It("reconnects to the socket if it fails at startup", func(done Done) {
			testMessage := "a very nice test message"
			readerChan := make(chan io.Reader)
			go loggingStream.FetchReader(readerChan)
			time.Sleep(150 * time.Millisecond) // ensure that reader tries to connect before socket is opened

			go sendMessageToSocket(socketPath, testMessage)

			reader := <-readerChan

			b, _ := ioutil.ReadAll(reader)

			Expect(string(b)).To(ContainSubstring(testMessage))
			logContents := loggertesthelper.TestLoggerSink.LogContents()
			Expect(string(logContents)).To(ContainSubstring("Could not read from socket OUT"))
			Expect(string(logContents)).To(ContainSubstring("Opened socket OUT"))

			close(done)
		})

		Context("with a socket already running", func() {
			var listener net.Listener
			var messagesToSend chan string

			BeforeEach(func() {
				listener, _ = net.Listen("unix", socketPath)
				messagesToSend = make(chan string)
				go func() {
					connection, _ := listener.Accept()
					defer connection.Close()
					for msg := range messagesToSend {
						loggertesthelper.Logger().Debugf("writing %s", msg)
						connection.Write([]byte(msg))
					}
				}()
			})

			AfterEach(func() {
				listener.Close()
				close(messagesToSend)
			})

			It("reads from the socket", func(done Done) {
				readerChan := make(chan io.Reader)
				go loggingStream.FetchReader(readerChan)
				reader := <-readerChan
				scanner := bufio.NewScanner(reader)

				for i := 0; i < 5; i++ {
					time.Sleep(100 * time.Millisecond)
					testMessage := fmt.Sprintf("Another Test Message %d", i)
					messagesToSend <- testMessage + "\n"

					message, _ := readLineFromScanner(scanner)

					Expect(string(message)).To(Equal(testMessage))
				}

				logContents := loggertesthelper.TestLoggerSink.LogContents()
				Expect(string(logContents)).To(ContainSubstring("Opened socket OUT"))
				close(done)
			}, 5)
		})
	})

	Describe("Stop", func() {
		Context("when connected", func() {
			var readerChan chan io.Reader
			BeforeEach(func() {
				readerChan = make(chan io.Reader)
				go sendMessageToSocket(socketPath, "don't care")
				go loggingStream.FetchReader(readerChan)
			})

			It("closes the listener connection", func() {
				loggingStream.Stop()
				buf := []byte{}
				reader := <-readerChan
				n, err := reader.Read(buf)
				Expect(n).To(BeZero())
				Expect(err).To(Equal(io.EOF))
			})
		})

		It("does not panic when called a second time", func() {
			loggingStream.Stop()
			Expect(loggingStream.Stop).NotTo(Panic())
		})
	})
})

func sendMessageToSocket(path, message string) {
	listener, _ := net.Listen("unix", path)
	defer listener.Close()
	connection, _ := listener.Accept()
	defer connection.Close()
	connection.Write([]byte(message + "\n"))
}

func readLineFromScanner(scanner *bufio.Scanner) (string, error) {
	scanner.Scan()
	if scanner.Err() == nil {
		return scanner.Text(), nil
	}

	return "", scanner.Err()
}
