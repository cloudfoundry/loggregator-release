package loggingstream_test

import (
	"deaagent/domain"
	"deaagent/loggingstream"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		loggingStream = loggingstream.NewLoggingStream(task, loggertesthelper.Logger(), logmessage.LogMessage_OUT)
		loggertesthelper.TestLoggerSink.Clear()
	})

	Describe("Listen", func() {
		It("should reconnect to the socket if it fails at startup", func(done Done) {
			testMessage := "a very nice test message"
			channel := loggingStream.Listen()

			go sendMessageToSocket(socketPath, testMessage)

			message := <-channel
			Expect(string(message.GetMessage())).To(Equal(testMessage))
			logContents := loggertesthelper.TestLoggerSink.LogContents()
			Expect(string(logContents)).To(ContainSubstring("Could not read from socket OUT"))
			Expect(string(logContents)).To(ContainSubstring("EOF while reading from socket OUT"))
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
					connection.Write([]byte("Test Message\n"))
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

			It("should read from the socket", func(done Done) {
				channel := loggingStream.Listen()

				message := <-channel
				Expect(string(message.GetMessage())).To(Equal("Test Message"))

				for i := 0; i < 5; i++ {
					time.Sleep(100 * time.Millisecond)
					testMessage := fmt.Sprintf("Another Test Message %d", i)
					messagesToSend <- testMessage + "\n"
					message := <-channel
					Expect(string(message.GetMessage())).To(Equal(testMessage))
				}

				logContents := loggertesthelper.TestLoggerSink.LogContents()
				Expect(string(logContents)).To(ContainSubstring("Opened socket OUT"))
				close(done)
			}, 5)

		})
	})

	Describe("Stop", func() {
		Context("when connected", func() {

			BeforeEach(func() {
				go sendMessageToSocket(socketPath, "don't care")
			})

			It("should shutdown the listener and close the channel", func() {
				channel := loggingStream.Listen()
				loggingStream.Stop()
				Eventually(channel).Should(BeClosed())
			})
		})

		Context("when never connected", func() {
			It("should shutdown the listener and close the channel", func() {
				channel := loggingStream.Listen()
				loggingStream.Stop()
				Eventually(channel, 2).Should(BeClosed())
			})
		})
	})
})

func sendMessageToSocket(path, message string) {
	time.Sleep(200 * time.Millisecond)
	listener, _ := net.Listen("unix", path)
	defer listener.Close()
	connection, _ := listener.Accept()
	defer connection.Close()
	connection.Write([]byte(message + "\n"))
}
