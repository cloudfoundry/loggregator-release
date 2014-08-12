package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"io/ioutil"
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var (
	tmpdir   string
	filePath string
)

var _ = Describe("DeaAgent", func() {

	var (
		task1StdoutListener    net.Listener
		task2StdoutListener    net.Listener
		expectedMessage        = "Some Output"
		mockLoggregatorEmitter MockLoggregatorEmitter
		agent                  *deaagent.Agent
		appUpdateChan          chan appservice.AppServices
	)

	BeforeEach(func() {
		var err error
		tmpdir, err = ioutil.TempDir("", "testing")
		if err != nil {
			panic(err)
		}
		filePath = tmpdir + "/instances.json"

		helperTask1 := &domain.Task{
			ApplicationId:       "1234",
			SourceName:          "App",
			WardenJobId:         56,
			WardenContainerPath: tmpdir,
			Index:               3,
		}

		var task1StderrListener net.Listener
		task1StdoutListener, task1StderrListener = setupTaskSockets(helperTask1)
		defer task1StderrListener.Close()

		helperTask2 := &domain.Task{
			ApplicationId:       "5678",
			SourceName:          "App",
			WardenJobId:         58,
			WardenContainerPath: tmpdir,
			Index:               0,
		}

		var task2StderrListener net.Listener
		task2StdoutListener, task2StderrListener = setupTaskSockets(helperTask2)
		defer task2StderrListener.Close()

		mockLoggregatorEmitter = MockLoggregatorEmitter{}

		mockLoggregatorEmitter.received = make(chan *events.LogMessage, 2)

		writeToFile(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3, "syslog_drain_urls": ["url1"]},
	                                {"state": "RUNNING", "application_id": "3456", "warden_job_id": 59, "warden_container_path":"`+tmpdir+`", "instance_index": 1}]}`, true)

		appUpdateChan = make(chan appservice.AppServices, 5)
		agent = deaagent.NewAgent(filePath, loggertesthelper.Logger(), appUpdateChan)
	})

	AfterEach(func() {
		task1StdoutListener.Close()
		task2StdoutListener.Close()
	})

	Describe("instances.json polling", func() {
		Context("at startup", func() {
			It("picks up new tasks", func() {
				agent.Start(mockLoggregatorEmitter)

				task1Connection, _ := task1StdoutListener.Accept()
				defer task1Connection.Close()

				task1Connection.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))

				msg := <-mockLoggregatorEmitter.received
				Expect(msg.GetAppId()).To(Equal("1234"))
			})
		})

		Context("while running", func() {
			It("picks up new tasks", func() {
				agent.Start(mockLoggregatorEmitter)

				writeToFile(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3, "syslog_drain_urls": ["url1"]},
								   {"state": "RUNNING", "application_id": "5678", "warden_job_id": 58, "warden_container_path":"`+tmpdir+`", "instance_index": 0, "syslog_drain_urls": ["url2"]},
	                               {"state": "RUNNING", "application_id": "1234", "warden_job_id": 57, "warden_container_path":"`+tmpdir+`", "instance_index": 2, "syslog_drain_urls": ["url1"]}
	                               ]}`, true)

				connectionChannel := make(chan net.Conn)
				go func() {
					task2Connection, _ := task2StdoutListener.Accept()
					connectionChannel <- task2Connection
				}()
				var task2Connection net.Conn
				select {
				case task2Connection = <-connectionChannel:
					defer task2Connection.Close()
				case <-time.After(1 * time.Second):
					Fail("Should have been able to open the socket listener")
				}

				task2Connection.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))

				msg := <-mockLoggregatorEmitter.received
				Expect(msg.GetAppId()).To(Equal("5678"))
			})
		})
	})

	It("creates the correct structure on forwarded messages and does not contain drain URLs", func() {
		agent.Start(mockLoggregatorEmitter)

		task1Connection, _ := task1StdoutListener.Accept()
		defer task1Connection.Close()

		task1Connection.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))

		receivedMessage := <-mockLoggregatorEmitter.received

		Expect(receivedMessage.GetSourceType()).To(Equal("App"))
		Expect(receivedMessage.GetMessageType()).To(Equal(events.LogMessage_OUT))
		Expect(string(receivedMessage.GetMessage())).To(Equal(expectedMessage))
	})

	It("pushes updates to syslog drain URLs to the appStoreUpdateChan", func() {
		agent.Start(mockLoggregatorEmitter)

		updates := make([]appservice.AppServices, 2)
		for i := 0; i < 2; i++ {
			updates[i] = <-appUpdateChan
		}

		expectedUpdates := []appservice.AppServices{
			{AppId: "1234", Urls: []string{"url1"}},
			{AppId: "3456", Urls: []string{}},
		}

		Expect(updates).To(ConsistOf(expectedUpdates))
	})
})

func writeToFile(text string, truncate bool) {
	file := createFile()
	defer file.Close()

	if truncate {
		file.Truncate(0)
	}

	file.WriteString(text)
}

func createFile() *os.File {
	file, _ := os.Create(filePath)
	return file
}
