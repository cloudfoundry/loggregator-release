package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testLogger = loggertesthelper.Logger()

var _ = Describe("TaskListener", func() {
	It("listens to stdout unix socket", func() {
		task, tmpdir := setupTask(3)
		defer os.RemoveAll(tmpdir)

		stdoutListener, stderrListener := setupTaskSockets(task)
		defer stdoutListener.Close()
		defer stderrListener.Close()

		emitter, receiveChannel := setupEmitter()
		taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
		go taskListner.StartListening()

		expectedMessage := "Some Output"
		secondLogMessage := "toally different"

		connection, err := stdoutListener.Accept()
		defer connection.Close()
		Expect(err).NotTo(HaveOccurred())

		_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())

		receivedMessage := <-receiveChannel

		Expect(receivedMessage.GetAppId()).To(Equal("1234"))
		Expect(receivedMessage.GetSourceName()).To(Equal("App"))
		Expect(receivedMessage.GetMessageType()).To(Equal(logmessage.LogMessage_OUT))
		Expect(string(receivedMessage.GetMessage())).To(Equal(expectedMessage))
		Expect(receivedMessage.GetDrainUrls()).To(Equal([]string{"syslog://10.20.30.40:8050"}))
		Expect(receivedMessage.GetSourceId()).To(Equal("3"))

		_, err = connection.Write([]byte(secondLogMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())

		receivedMessage = <-receiveChannel

		Expect(receivedMessage.GetAppId()).To(Equal("1234"))
		Expect(receivedMessage.GetSourceName()).To(Equal("App"))
		Expect(receivedMessage.GetMessageType()).To(Equal(logmessage.LogMessage_OUT))
		Expect(string(receivedMessage.GetMessage())).To(Equal(secondLogMessage))
		Expect(receivedMessage.GetDrainUrls()).To(Equal([]string{"syslog://10.20.30.40:8050"}))
		Expect(receivedMessage.GetSourceId()).To(Equal("3"))

		_, err = connection.Write([]byte("a single line\nthat should be split\n"))
		Expect(err).NotTo(HaveOccurred())

		receivedMessage = <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal("a single line"))
		receivedMessage = <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal("that should be split"))
	})

	It("handles four byte offset", func() {
		task, tmpdir := setupTask(3)
		defer os.RemoveAll(tmpdir)

		stdoutListener, stderrListener := setupTaskSockets(task)
		defer stdoutListener.Close()
		defer stderrListener.Close()

		emitter, receiveChannel := setupEmitter()
		taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
		go taskListner.StartListening()

		expectedMessage := "Some Output"
		secondLogMessage := "toally different"

		connection, err := stdoutListener.Accept()
		defer connection.Close()

		Expect(err).NotTo(HaveOccurred())

		_, err = connection.Write([]byte("\n"))
		_, err = connection.Write([]byte("\n"))

		select {
		case _ = <-receiveChannel:
			Fail("Should not receive a message")
		case <-time.After(200 * time.Millisecond):
		}

		_, err = connection.Write([]byte("\n\n"))
		_, err = connection.Write([]byte(expectedMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())

		receivedMessage := <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal(expectedMessage))

		_, err = connection.Write([]byte(secondLogMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())

		receivedMessage = <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal(secondLogMessage))
	})

	It("listens to stderr unix socket", func() {
		task, tmpdir := setupTask(4)
		defer os.RemoveAll(tmpdir)

		stdoutListener, stderrListener := setupTaskSockets(task)
		defer stdoutListener.Close()
		defer stderrListener.Close()

		expectedMessage := "Some Output"
		secondLogMessage := "toally different"

		emitter, receiveChannel := setupEmitter()
		taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
		go taskListner.StartListening()

		connection, err := stderrListener.Accept()
		defer connection.Close()
		Expect(err).NotTo(HaveOccurred())

		_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())
		receivedMessage := <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal(expectedMessage))

		_, err = connection.Write([]byte(secondLogMessage))
		_, err = connection.Write([]byte("\n"))
		Expect(err).NotTo(HaveOccurred())
		receivedMessage = <-receiveChannel
		Expect(string(receivedMessage.GetMessage())).To(Equal(secondLogMessage))
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
		DrainUrls:           []string{"syslog://10.20.30.40:8050"}}

	os.MkdirAll(appTask.Identifier(), 0777)

	return appTask, tmpdir
}
