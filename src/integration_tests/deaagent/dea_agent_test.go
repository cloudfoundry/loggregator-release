package integration_test

import (
	"deaagent/domain"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/onsi/gomega/gexec"
	"testing"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/onsi/ginkgo/config"

	"fmt"
	"github.com/gogo/protobuf/proto"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

const natsPort = 24484
const SOCKET_PREFIX = "\n\n\n\n"
const instancesJsonPath = "fixtures/instances.json"
const wardenIdentifier = 56

var (
	task1InputListener  net.Listener
	task1StderrListener net.Listener
	newFile             *os.File
	etcdPort            int
	etcdRunner          *etcdstorerunner.ETCDClusterRunner
	deaAgentSession     *gexec.Session
	goRoutineSpawned    sync.WaitGroup
)

type messageHelperInterface interface {
	getTotalNumOfOutputMessages() int
	getValueOfValueMetric(string) float64
	getFirstLogMessageString() string
}

type messageHolder struct {
	messages []events.Envelope
	sync.RWMutex
}

var _ = BeforeSuite(func() {
	newFile = createFile(instancesJsonPath)
	fileContent := fmt.Sprintf(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": %d, "warden_container_path":"fixtures", "instance_index": 3, "syslog_drain_urls": ["url1"]}]}`, wardenIdentifier)
	writeToFile(newFile, fileContent, true)

	helperTask1 := &domain.Task{
		ApplicationId:       "1234",
		SourceName:          "App",
		WardenJobId:         wardenIdentifier,
		WardenContainerPath: "fixtures",
		Index:               3,
	}

	task1InputListener, task1StderrListener = setupTaskSockets(helperTask1)

	pathToDeaAgentExecutable, err := gexec.Build("deaagent/deaagent")
	Expect(err).ShouldNot(HaveOccurred())

	deaagentCommand := exec.Command(pathToDeaAgentExecutable, "--config=fixtures/deaagent.json", "--debug", "-instancesFile", "fixtures/instances.json")

	deaAgentSession, err = gexec.Start(deaagentCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).ShouldNot(HaveOccurred())
	newFile, err = os.OpenFile("fixtures/instances.json", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		println(err.Error())
	}

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
})

var _ = AfterSuite(func() {
	task1InputListener.Close()
	task1StderrListener.Close()
	deaAgentSession.Kill().Wait(5)
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
	os.Remove(instancesJsonPath)
	os.RemoveAll("fixtures/jobs")
})

var _ = Describe("DeaLoggingAgent integration tests", func() {
	Context("It sends messages from the input file out to metron", func() {
		It("gets sent messages and internally emitted metrics", func(done Done) {
			m := &messageHolder{}
			conn := makeDeaLoggingAgentOutputConn()
			defer conn.Close()
			goRoutineSpawned.Add(1)
			go readDeaLoggingAgentOutputConn(conn, m)
			goRoutineSpawned.Wait()
			//Send a message into dea logging agent
			task1StdoutConn, err := task1InputListener.Accept()
			Expect(err).ToNot(HaveOccurred())
			defer task1StdoutConn.Close()
			task1StdoutConn.Write([]byte(SOCKET_PREFIX + "some message" + "\n"))

			Eventually(m.getTotalNumOfOutputMessages, 20, 1).Should(BeNumerically(">", 10))
			Expect(m.getValueOfValueMetric("logSenderTotalMessagesRead")).To(Equal(float64(1)))

			Expect(m.getFirstLogMessageString()).To(Equal("some message"))

			newFile.Close()
			close(done)
		}, 30)
	})
})

func (m *messageHolder) getTotalNumOfOutputMessages() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.messages)
}

func (m *messageHolder) getValueOfValueMetric(metricName string) float64 {
	m.RLock()
	defer m.RUnlock()
	for _, metric := range m.messages {
		if *metric.EventType == events.Envelope_ValueMetric && metric.ValueMetric.GetName() == metricName {
			return metric.ValueMetric.GetValue()
		}
	}

	return -1
}

func (m *messageHolder) getFirstLogMessageString() string {
	m.RLock()
	defer m.RUnlock()
	for _, message := range m.messages {
		if *message.EventType == events.Envelope_LogMessage {
			return string(message.GetLogMessage().GetMessage())
		}
	}
	return ""
}

func setupTaskSockets(task *domain.Task) (stdout net.Listener, stderr net.Listener) {
	os.MkdirAll(task.Identifier(), 0777)
	stdoutSocketPath := filepath.Join(task.Identifier(), "stdout.sock")
	os.Remove(stdoutSocketPath)
	stdoutListener, _ := net.Listen("unix", stdoutSocketPath)

	stderrSocketPath := filepath.Join(task.Identifier(), "stderr.sock")
	os.Remove(stderrSocketPath)
	stderrListener, _ := net.Listen("unix", stderrSocketPath)
	return stdoutListener, stderrListener
}

func writeToFile(newFile *os.File, text string, truncate bool) {
	_, err := newFile.WriteString(text)
	Expect(err).ToNot(HaveOccurred())
}

func createFile(path string) *os.File {
	file, _ := os.Create(path)
	return file
}

func makeDeaLoggingAgentOutputConn() net.PacketConn {
	connection, err := net.ListenPacket("udp", "127.0.0.1:51161")
	Expect(err).ToNot(HaveOccurred())
	return connection
}

func readDeaLoggingAgentOutputConn(connection net.PacketConn, m *messageHolder) {
	readBuffer := make([]byte, 65535) //buffer with size = max theoretical UDP size
	goRoutineSpawned.Done()
	for {
		readCount, _, err := connection.ReadFrom(readBuffer)
		if err != nil {
			return
		}
		readData := make([]byte, readCount) //pass on buffer in size only of read data
		copy(readData, readBuffer[:readCount])
		envelope := UnmarshalMessage(readData)
		m.Lock()
		m.messages = append(m.messages, envelope)
		m.Unlock()
	}
}

func UnmarshalMessage(messageBytes []byte) events.Envelope {
	var envelope events.Envelope
	err := proto.Unmarshal(messageBytes, &envelope)
	Expect(err).NotTo(HaveOccurred())
	return envelope
}
