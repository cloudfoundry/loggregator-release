package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	tmpdir   string
	filePath string
)

var _ = Describe("DeaAgent", func() {
	var (
		task1StdoutListener net.Listener
		task1StderrListener net.Listener
		task2StdoutListener net.Listener
		task2StderrListener net.Listener
		expectedMessage     = "Some Output"
		agent               *deaagent.Agent
		fakeMetricSender    *fake.FakeMetricSender
	)

	BeforeEach(func() {
		fakeLogSender.Reset()

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

		task1StdoutListener, task1StderrListener = setupTaskSockets(helperTask1)

		helperTask2 := &domain.Task{
			ApplicationId:       "3456",
			SourceName:          "App",
			WardenJobId:         59,
			WardenContainerPath: tmpdir,
			Index:               1,
		}

		task2StdoutListener, task2StderrListener = setupTaskSockets(helperTask2)

		writeToFile(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3, "syslog_drain_urls": ["url1"]},
	                                {"state": "RUNNING", "application_id": "3456", "warden_job_id": 59, "warden_container_path":"`+tmpdir+`", "instance_index": 1}]}`, true)

		agent = deaagent.NewAgent(filePath, loggertesthelper.Logger())

		fakeMetricSender = fake.NewFakeMetricSender()
		metrics.Initialize(fakeMetricSender, metricbatcher.New(fakeMetricSender, 10*time.Millisecond))
	})

	AfterEach(func() {
		task1StdoutListener.Close()
		task1StderrListener.Close()
		task2StdoutListener.Close()
		task2StderrListener.Close()
		agent.Stop()
	})

	Describe("instances.json polling", func() {
		Context("at startup", func() {
			It("picks up new tasks", func() {
				agent.Start()
				task1StdoutConn, _ := task1StdoutListener.Accept()
				defer task1StdoutConn.Close()
				task1StderrConn, _ := task1StderrListener.Accept()
				defer task1StderrConn.Close()
				task1StdoutConn.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))
				Eventually(fakeLogSender.GetLogs).Should(HaveLen(1))
				logs := fakeLogSender.GetLogs()
				Expect(logs[0].AppId).To(Equal("1234"))
			})
		})

		Context("while running", func() {
			BeforeEach(func() {
				agent.Start()

				writeToFile(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3, "syslog_drain_urls": ["url1"]},
								{"state": "RUNNING", "application_id": "3456", "warden_job_id": 59, "warden_container_path":"`+tmpdir+`", "instance_index": 1},
								{"state": "RUNNING", "application_id": "5678", "warden_job_id": 58, "warden_container_path":"`+tmpdir+`", "instance_index": 0, "syslog_drain_urls": ["url2"]}
							   ]}`, true)
			})

			It("picks up new tasks", func() {
				connectionChannel := make(chan net.Conn)

				newTask := &domain.Task{
					ApplicationId:       "5678",
					SourceName:          "App",
					WardenJobId:         58,
					WardenContainerPath: tmpdir,
					Index:               0,
				}

				newTaskStdoutListener, newTaskStderrListener := setupTaskSockets(newTask)

				go func() {
					newTaskConnection, _ := newTaskStdoutListener.Accept()
					connectionChannel <- newTaskConnection
				}()

				go newTaskStderrListener.Accept()

				var newTaskConnection net.Conn
				select {
				case newTaskConnection = <-connectionChannel:
					defer newTaskConnection.Close()
				case <-time.After(1 * time.Second):
					Fail("Should have been able to open the socket listener")
				}

				newTaskConnection.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))

				Eventually(fakeLogSender.GetLogs, 3).Should(HaveLen(1))
				logs := fakeLogSender.GetLogs()
				Expect(logs[0].AppId).To(Equal("5678"))
			})

			Context("metrics", func() {
				It("updates totalApps", func() {
					Eventually(func() float64 {
						return fakeMetricSender.GetValue("totalApps").Value
					}).Should(BeEquivalentTo(3))

					deleteFile()

					Eventually(func() float64 {
						return fakeMetricSender.GetValue("totalApps").Value
					}, 3).Should(BeEquivalentTo(0))
				})
			})
		})

	})

	It("creates the correct structure on forwarded messages and does not contain drain URLs", func() {
		agent.Start()

		task1Connection, _ := task1StdoutListener.Accept()
		defer task1Connection.Close()

		task1Connection.Write([]byte(SOCKET_PREFIX + expectedMessage + "\n"))

		Eventually(fakeLogSender.GetLogs).Should(HaveLen(1))
		logs := fakeLogSender.GetLogs()
		Expect(logs[0].SourceType).To(Equal("App"))
		Expect(logs[0].MessageType).To(Equal("OUT"))
		Expect(logs[0].Message).To(Equal(expectedMessage))
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

func deleteFile() {
	os.Remove(filePath)
}
