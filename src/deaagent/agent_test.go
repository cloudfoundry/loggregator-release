package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"io/ioutil"
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
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
		fakeSyslogDrainStore   *FakeSyslogDrainStore
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

		fakeSyslogDrainStore = NewFakeSyslogDrainStore()
		agent = deaagent.NewAgent(filePath, loggertesthelper.Logger(), fakeSyslogDrainStore,
			10*time.Millisecond, 10*time.Millisecond)
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

	Describe("Refreshing app TTLs", func() {
		It("periodically refreshes TTLs for app nodes", func() {
			agent.Start(mockLoggregatorEmitter)

			Eventually(func() int { return fakeSyslogDrainStore.AppNodeCallCount("1234") }).Should(BeNumerically(">", 1))
			Eventually(func() int { return fakeSyslogDrainStore.AppNodeCallCount("3456") }).Should(BeNumerically(">", 1))
		})
	})

	Describe("refreshing drain URLs in etcd to recover in case of etcd failure", func() {
		It("periodically updates the drain store", func() {
			agent.Start(mockLoggregatorEmitter)

			Eventually(func() int { return len(fakeSyslogDrainStore.UpdateDrainCalls()) }).Should(BeNumerically(">", 2))
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

	It("pushes updates to syslog drain URLs to the syslog drain store", func() {
		agent.Start(mockLoggregatorEmitter)

		expectedUpdates := []updateDrainParams{
			{appId: "1234", drainUrls: []string{"url1"}},
			{appId: "3456", drainUrls: []string{}},
		}

		Eventually(fakeSyslogDrainStore.UpdateDrainCalls).Should(ConsistOf(expectedUpdates))
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

type FakeSyslogDrainStore struct {
	updateDrainCalls    []updateDrainParams
	refreshAppNodeCalls map[string]int
	sync.Mutex
}

func NewFakeSyslogDrainStore() *FakeSyslogDrainStore {
	return &FakeSyslogDrainStore{
		refreshAppNodeCalls: make(map[string]int),
	}
}

type updateDrainParams struct {
	appId     string
	drainUrls []string
}

func (s *FakeSyslogDrainStore) UpdateDrainCalls() []updateDrainParams {
	s.Lock()
	defer s.Unlock()
	return s.updateDrainCalls
}

func (s *FakeSyslogDrainStore) AppNodeCallCount(appId string) int {
	s.Lock()
	defer s.Unlock()
	return s.refreshAppNodeCalls[appId]
}

func (s *FakeSyslogDrainStore) UpdateDrains(appId string, drainUrls []string) error {
	s.Lock()
	defer s.Unlock()
	s.updateDrainCalls = append(s.updateDrainCalls, updateDrainParams{appId, drainUrls})
	return nil
}
func (s *FakeSyslogDrainStore) RefreshAppNode(appId string) error {
	s.Lock()
	defer s.Unlock()
	s.refreshAppNodeCalls[appId] += 1
	return nil
}
