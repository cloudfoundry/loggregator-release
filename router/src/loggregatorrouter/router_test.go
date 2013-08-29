package loggregatorrouter

import (
	"code.google.com/p/gogoprotobuf/proto"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var logger = steno.NewLogger("TestLogger")

func TestThatItWorksWithOneLoggregator(t *testing.T) {
	listener := agentlistener.NewAgentListener("localhost:9999", logger)
	dataChannel := listener.Start()

	loggregatorServers := []string{"localhost:9999"}
	h := NewRouter("localhost:3456", loggregatorServers, logger)
	go h.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3456", "ROUTER", logger)
	logEmitter.Emit("my_awesome_app", "Hello World")

	received := <-dataChannel
	receivedMessage := &logmessage.LogMessage{}
	proto.Unmarshal(received, receivedMessage)

	assert.Equal(t, receivedMessage.GetAppId(), "my_awesome_app")
	assert.Equal(t, string(receivedMessage.GetMessage()), "Hello World")
}

func TestThatItIgnoresBadMessages(t *testing.T) {
	listener := agentlistener.NewAgentListener("localhost:9996", logger)
	dataChannel := listener.Start()

	loggregatorServers := []string{"localhost:9996"}
	h := NewRouter("localhost:3455", loggregatorServers, logger)
	go h.Start(logger)
	time.Sleep(50 * time.Millisecond)

	lc := loggregatorclient.NewLoggregatorClient("localhost:3455", logger, loggregatorclient.DefaultBufferSize)
	lc.Send([]byte("This is poorly formatted"))

	logEmitter, _ := emitter.NewEmitter("localhost:3455", "ROUTER", logger)
	logEmitter.Emit("my_awesome_app", "Hello World")

	received := <-dataChannel
	receivedMessage := &logmessage.LogMessage{}
	proto.Unmarshal(received, receivedMessage)

	assert.Equal(t, receivedMessage.GetAppId(), "my_awesome_app")
	assert.Equal(t, string(receivedMessage.GetMessage()), "Hello World")
}

func TestThatItWorksWithTwoLoggregators(t *testing.T) {
	listener1 := agentlistener.NewAgentListener("localhost:9998", logger)
	dataChan1 := listener1.Start()

	listener2 := agentlistener.NewAgentListener("localhost:9997", logger)
	dataChan2 := listener2.Start()

	loggregatorServers := []string{"localhost:9998", "localhost:9997"}
	rt := NewRouter("localhost:3457", loggregatorServers, logger)
	go rt.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3457", "ROUTER", logger)
	logEmitter.Emit("testId", "My message")

	receivedData := <-dataChan1
	receivedMsg := &logmessage.LogMessage{}
	proto.Unmarshal(receivedData, receivedMsg)

	assert.Equal(t, receivedMsg.GetAppId(), "testId")
	assert.Equal(t, string(receivedMsg.GetMessage()), "My message")

	logEmitter.Emit("anotherId", "Another message")

	receivedData = <-dataChan2
	receivedMsg = &logmessage.LogMessage{}
	proto.Unmarshal(receivedData, receivedMsg)

	assert.Equal(t, receivedMsg.GetAppId(), "anotherId")
	assert.Equal(t, string(receivedMsg.GetMessage()), "Another message")
}

func TestThatEmitWorks(t *testing.T) {
	loggregatorServers := []string{"127.0.0.2:9998", "127.0.0.3:9997"}
	rt := NewRouter("127.0.0.1:3457", loggregatorServers, logger)
	context := rt.Emit()
	assert.Equal(t, "loggregatorRouter", context.Name)
	metrics := context.Metrics
	assert.Equal(t, len(metrics), 17) //make sure all expected metrics are present
	for _, metric := range metrics {
		switch metric.Name {
		case "127:0:0:1.currentBufferCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:1.receivedMessageCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:1.receivedByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.currentBufferCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.sentMessageCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.sentByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.receivedMessageCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.receivedByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.logStreamRawByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:2.logStreamPbByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.currentBufferCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.sentMessageCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.sentByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.receivedMessageCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.receivedByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.logStreamRawByteCount":
			assert.NotNil(t, metric.Value)
		case "127:0:0:3.logStreamPbByteCount":
			assert.NotNil(t, metric.Value)
		default:
			t.Error("Got an invalid metric name: ", metric.Name)
		}
	}
}
