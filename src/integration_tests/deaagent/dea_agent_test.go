package deaagent_test

import (
	"deaagent/domain"

	"github.com/cloudfoundry/sonde-go/events"

	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const SOCKET_PREFIX = "\n\n\n\n"

var (
	goRoutineSpawned sync.WaitGroup
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

var _ = Describe("DeaLoggingAgent integration tests", func() {
	Context("It sends messages from the input file out to metron", func() {
		It("gets sent messages and internally emitted metrics", func() {
			m := &messageHolder{}
			conn := makeDeaLoggingAgentOutputConn()
			defer conn.Close()
			goRoutineSpawned.Add(1)
			go readDeaLoggingAgentOutputConn(conn, m)
			goRoutineSpawned.Wait()

			stdoutChannel := make(chan net.Conn)
			//Send a message into dea logging agent
			go func() {
				defer GinkgoRecover()
				stdoutConn, err := task1InputListener.Accept()
				Expect(err).NotTo(HaveOccurred())
				stdoutChannel <- stdoutConn
			}()
			var task1StdoutConn net.Conn
			Eventually(stdoutChannel).Should(Receive(&task1StdoutConn))
			defer task1StdoutConn.Close()

			Eventually(m.getMessagesCount, 10, 1).Should(BeNumerically(">", 5))
			task1StdoutConn.Write([]byte(SOCKET_PREFIX + "some message" + "\n"))
			Eventually(m.getCounterCount, 10, 1).Should(BeNumerically(">", 0))

			Expect(m.getDeltaOfCounterEvent("logSenderTotalMessagesRead")).To(Equal(uint64(1)))

			Expect(m.getFirstLogMessageString()).To(Equal("some message"))
		}, 25)
	})
})

func (m *messageHolder) getMessagesCount() int {
	defer m.RUnlock()
	m.RLock()
	return len(m.messages)
}

func (m *messageHolder) getCounterCount() int {
	count := 0
	m.RLock()
	defer m.RUnlock()
	for _, metric := range m.messages {
		if *metric.EventType == events.Envelope_CounterEvent {
			count++
		}
	}
	return count
}

func (m *messageHolder) getDeltaOfCounterEvent(metricName string) uint64 {
	m.RLock()
	defer m.RUnlock()
	for _, metric := range m.messages {
		if *metric.EventType == events.Envelope_CounterEvent && metric.CounterEvent.GetName() == metricName {
			return metric.CounterEvent.GetDelta()
		}
	}

	return 0
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
	err := os.MkdirAll(task.Identifier(), 0777)
	Expect(err).NotTo(HaveOccurred())
	stdoutSocketPath := filepath.Join(task.Identifier(), "stdout.sock")
	_ = os.Remove(stdoutSocketPath)

	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	Expect(err).NotTo(HaveOccurred())

	stderrSocketPath := filepath.Join(task.Identifier(), "stderr.sock")
	_ = os.Remove(stderrSocketPath)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	Expect(err).NotTo(HaveOccurred())

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
	connection, err := net.ListenPacket("udp4", "127.0.0.1:51161")
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
