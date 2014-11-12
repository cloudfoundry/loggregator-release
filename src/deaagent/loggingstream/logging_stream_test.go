package loggingstream_test

import (
	"deaagent/domain"
	"deaagent/loggingstream"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Describe("Reading from the stream", func() {
		Context("When the socket is open", func() {
			BeforeEach(func() {
				listener, _ := net.Listen("unix", socketPath)
				go func() {
					connection, _ := listener.Accept()
					connection.Write([]byte("Hello World!"))
					connection.Write([]byte("Goodbye World!"))
					defer connection.Close()
				}()
			})

			It("reads the information from the socket multiple times", func() {
				p := make([]byte, len("Hello World!"))
				count, err := loggingStream.Read(p)

				Expect(err).To(BeNil())
				Expect(count).To(Equal(len("Hello World!")))
				Expect(string(p)).To(ContainSubstring("Hello World!"))

				p = make([]byte, len("Goodbye World!"))

				count, err = readWithTimeout(1*time.Second, p, loggingStream)

				Expect(err).To(BeNil())
				Expect(count).To(Equal(len("Goodbye World!")))
				Expect(string(p)).To(ContainSubstring("Goodbye World!"))
			})
		})

		Context("When the socket is closed by the app", func() {
			BeforeEach(func() {
				listener, _ := net.Listen("unix", socketPath)
				go func() {
					connection, _ := listener.Accept()
					defer connection.Close()
					defer listener.Close()
				}()
			})

			It("you get an EOF", func() {
				p := make([]byte, 1024)
				_, err := loggingStream.Read(p)

				Expect(err).To(Equal(io.EOF))
			})

			It("tries to reconnect after the first connection has closed", func() {
				p := make([]byte, 1024)
				_, err := loggingStream.Read(p)
				Expect(err).To(Equal(io.EOF))

				go func() {
					listener, _ := net.Listen("unix", socketPath)
					connection, _ := listener.Accept()
					connection.Write([]byte("Hello"))
					defer connection.Close()
					defer listener.Close()
				}()

				n, err := loggingStream.Read(p)
				Expect(n).To(Equal(5))
				Expect(err).ToNot(HaveOccurred())

				_, err = loggingStream.Read(p)
				Expect(err).To(Equal(io.EOF))
			})
		})

		Context("when socket never opens", func() {
			It("returns an EOF error and 0 bytes ", func() {
				p := make([]byte, 1024)
				count, err := loggingStream.Read(p)
				Expect(err).To(Equal(io.EOF))
				Expect(count).To(Equal(0))
			})
		})
	})

	Describe("Close", func() {
		Context("after read is called", func() {
			It("closes the socket connection", func() {
				listener, _ := net.Listen("unix", socketPath)
				var connection net.Conn
				go func() {
					connection, _ = listener.Accept()
					connection.Write([]byte("Hello World!"))
				}()

				p := make([]byte, 1024)
				loggingStream.Read(p)

				loggingStream.Close()

				_, err := connection.Write([]byte("Hello World!"))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("if read is not called", func() {
			It("does not panic", func() {
				listener, _ := net.Listen("unix", socketPath)
				var connection net.Conn
				go func() {
					connection, _ = listener.Accept()
				}()

				loggingStream.Close()
			})
		})

		Context("while read is listening", func() {
			It("closes the ongoing read", func(done Done) {
				listener, _ := net.Listen("unix", socketPath)
				var connection net.Conn
				go func() {
					connection, _ = listener.Accept()
				}()

				readDone := make(chan struct{})
				go func() {
					p := make([]byte, 1024)
					loggingStream.Read(p)
					close(readDone)
				}()

				time.Sleep(100 * time.Millisecond) // wait for Read to get to a blocking point

				loggingStream.Close()
				Eventually(readDone).Should(BeClosed())
				close(done)
			}, 2)
		})
	})
})

func readWithTimeout(duration time.Duration, p []byte, reader io.Reader) (int, error) {
	doneChan := make(chan struct{})

	var count int
	var err error

	go func() {
		count, err = reader.Read(p)
		close(doneChan)
	}()

	select {
	case <-time.After(duration):
		panic("Read timed out")
	case <-doneChan:
		return count, err
	}
}
