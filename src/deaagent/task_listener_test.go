package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"io/ioutil"
	"net"
	"os"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testLogger = loggertesthelper.Logger()

var _ = Describe("TaskListener", func() {
	Describe("StartListening", func() {
		var task *domain.Task
		var tmpdir string
		var stdoutListener, stderrListener net.Listener
		var stdoutConnection, stderrConnection net.Conn
		var message1 = "one"
		var message2 = "two"
		var taskListener *deaagent.TaskListener

		BeforeEach(func() {
			fakeLogSender.Reset()
			task, tmpdir = setupTask(3)

			stdoutListener, stderrListener = setupTaskSockets(task)

			taskListener, _ = deaagent.NewTaskListener(*task, testLogger)
			go taskListener.StartListening()

			var err error
			stdoutConnection, err = stdoutListener.Accept()
			Expect(err).NotTo(HaveOccurred())

			stderrConnection, err = stderrListener.Accept()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpdir)
			stdoutListener.Close()
			stderrListener.Close()
			stdoutConnection.Close()
			stderrConnection.Close()
			taskListener.StopListening()
		})

		It("receives single line message sent to STDOUT", func() {
			stdoutConnection.Write([]byte(SOCKET_PREFIX + message1 + "\n"))

			Eventually(fakeLogSender.GetLogs).Should(HaveLen(1))

			log := fakeLogSender.GetLogs()[0]
			Expect(log.AppId).To(Equal("1234"))
			Expect(log.SourceType).To(Equal("App"))
			Expect(log.Message).To(Equal(message1))
			Expect(log.SourceInstance).To(Equal("3"))
			Expect(log.MessageType).To(Equal("OUT"))

			stdoutConnection.Write([]byte(SOCKET_PREFIX + message2 + "\n"))

			Eventually(fakeLogSender.GetLogs).Should(HaveLen(2))

			log = fakeLogSender.GetLogs()[1]
			Expect(log.Message).To(Equal(message2))
		})

		It("receives multiline messages by line", func() {
			stdoutConnection.Write([]byte(SOCKET_PREFIX + message1 + "\n" + message2 + "\n"))

			Eventually(fakeLogSender.GetLogs).Should(HaveLen(2))

			Expect(fakeLogSender.GetLogs()[0].Message).To(Equal(message1))
			Expect(fakeLogSender.GetLogs()[1].Message).To(Equal(message2))
		})

		It("receives single line message sent to STDERR", func() {
			stderrConnection.Write([]byte(SOCKET_PREFIX + message1 + "\n"))

			Eventually(fakeLogSender.GetLogs).Should(HaveLen(1))

			log := fakeLogSender.GetLogs()[0]
			Expect(log.AppId).To(Equal("1234"))
			Expect(log.SourceType).To(Equal("App"))
			Expect(log.Message).To(Equal(message1))
			Expect(log.SourceInstance).To(Equal("3"))
			Expect(log.MessageType).To(Equal("ERR"))

			stderrConnection.Write([]byte(SOCKET_PREFIX + message2 + "\n"))

			Eventually(fakeLogSender.GetLogs).Should(HaveLen(2))

			log = fakeLogSender.GetLogs()[1]
			Expect(log.Message).To(Equal(message2))
		})
	})

	Describe("Initilization", func() {
		Context("Both sockets available", func() {
			It("keeps both connections open", func() {
				task, _ := setupTask(3)

				stdoutListener, stderrListener := setupTaskSockets(task)

				connectionChannel := make(chan net.Conn)
				go func() {
					connection, _ := stdoutListener.Accept()
					connectionChannel <- connection
				}()
				go func() {
					connection, _ := stderrListener.Accept()
					connectionChannel <- connection
				}()

				taskListener, err := deaagent.NewTaskListener(*task, testLogger)
				Expect(err).To(BeNil())
				Expect(taskListener).NotTo(BeNil())

				Eventually(connectionChannel).Should(Receive())
				Eventually(connectionChannel).Should(Receive())
			})
		})

		Context("Stdout socket unavailable", func() {
			It("closes both connections", func() {
				task, _ := setupTask(3)

				stdoutListener, stderrListener := setupTaskSockets(task)
				stdoutListener.Close()

				connectionChannel := make(chan net.Conn)

				go func() {
					connection, _ := stderrListener.Accept()
					connectionChannel <- connection
				}()

				taskListener, err := deaagent.NewTaskListener(*task, testLogger)
				Expect(err).ToNot(BeNil())
				Expect(taskListener).To(BeNil())

				Consistently(connectionChannel).ShouldNot(Receive())
			})
		})

		Context("Stderr socket unavailable", func() {
			It("closes both connections", func() {
				task, _ := setupTask(3)

				stdoutListener, stderrListener := setupTaskSockets(task)
				stderrListener.Close()

				connectionChannel := make(chan net.Conn)

				go func() {
					connection, _ := stdoutListener.Accept()
					connectionChannel <- connection
				}()

				taskListener, err := deaagent.NewTaskListener(*task, testLogger)
				Expect(err).ToNot(BeNil())
				Expect(taskListener).To(BeNil())
				connection := <-connectionChannel
				data := make([]byte, 1024)
				_, err = connection.Read(data)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("failure to stream logs", func() {
		var (
			stdOutConnectionChannel, stdErrConnectionChannel chan net.Conn
			doneListening                                    chan struct{}
		)

		BeforeEach(func() {
			fakeLogSender.Reset()

			task, _ := setupTask(3)

			stdoutListener, stderrListener := setupTaskSockets(task)

			stdOutConnectionChannel = make(chan net.Conn)
			stdErrConnectionChannel = make(chan net.Conn)

			go func() {
				connection, _ := stdoutListener.Accept()
				stdOutConnectionChannel <- connection
				for {
					_, err := connection.Write([]byte("hello stdout\n"))
					if err != nil {
						break
					}
				}
			}()

			go func() {
				connection, _ := stderrListener.Accept()
				stdErrConnectionChannel <- connection
				for {
					_, err := connection.Write([]byte("hello stderr\n"))
					if err != nil {
						break
					}
				}
			}()

			taskListener, err := deaagent.NewTaskListener(*task, testLogger)
			Expect(err).ToNot(HaveOccurred())

			doneListening = make(chan struct{})
			go func() {
				taskListener.StartListening()
				close(doneListening)
			}()
		})

		It("closes stderr when stdout fails", func(done Done) {
			stdOutConnection := <-stdOutConnectionChannel
			stdOutConnection.Close()
			stdErrConnection := <-stdErrConnectionChannel

			Eventually(doneListening).Should(BeClosed())
			p := make([]byte, 1024)
			_, err := stdErrConnection.Read(p)
			Expect(err).To(HaveOccurred())

			close(done)
		})

		It("closes stdout when stderr fails", func(done Done) {
			stdErrConnection := <-stdErrConnectionChannel
			stdErrConnection.Close()
			stdOutConnection := <-stdOutConnectionChannel

			Eventually(doneListening).Should(BeClosed())
			p := make([]byte, 1024)
			_, err := stdOutConnection.Read(p)
			Expect(err).To(HaveOccurred())

			close(done)
		})
	})
})

func setupTask(index uint64) (appTask *domain.Task, tmpdir string) {
	tmpdir, err := ioutil.TempDir("", "testing")
	Expect(err).NotTo(HaveOccurred())

	appTask = &domain.Task{
		ApplicationId:       "1234",
		WardenJobId:         56,
		WardenContainerPath: tmpdir,
		Index:               index,
		SourceName:          "App",
	}

	os.MkdirAll(appTask.Identifier(), 0777)

	return appTask, tmpdir
}
