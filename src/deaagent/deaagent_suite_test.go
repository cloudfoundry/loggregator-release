package deaagent_test

import (
	"deaagent/domain"
	"github.com/cloudfoundry/dropsonde/emitter/logemitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/log_sender/fake"
	"github.com/cloudfoundry/dropsonde/logs"
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestDeaagent(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		fakeLogSender = fake.NewFakeLogSender()
		logs.Initialize(fakeLogSender)
	})

	RunSpecs(t, "Deaagent Suite")
}

var fakeLogSender *fake.FakeLogSender

const SOCKET_PREFIX = "\n\n\n\n"

type MockLoggregatorEmitter struct {
	received chan *events.LogMessage
}

func (m MockLoggregatorEmitter) Emit(a, b string) {

}

func (m MockLoggregatorEmitter) EmitError(a, b string) {

}

func (m MockLoggregatorEmitter) EmitLogMessage(message *events.LogMessage) {
	m.received <- message
}

func setupEmitter() (logemitter.Emitter, chan *events.LogMessage) {
	mockLoggregatorEmitter := new(MockLoggregatorEmitter)
	mockLoggregatorEmitter.received = make(chan *events.LogMessage)
	return mockLoggregatorEmitter, mockLoggregatorEmitter.received
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
