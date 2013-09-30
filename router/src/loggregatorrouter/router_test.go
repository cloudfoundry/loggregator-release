package loggregatorrouter

import (
	"code.google.com/p/gogoprotobuf/proto"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"loggregatorrouter/hasher"
	"testing"
	"time"
)

var logger = steno.NewLogger("TestLogger")

func newCfConfig() cfcomponent.Config {
	return cfcomponent.Config{}
}

func TestThatItWorksWithOneLoggregator(t *testing.T) {
	listener := agentlistener.NewAgentListener("localhost:9999", logger)
	dataChannel := listener.Start()

	loggregatorServers := []string{"localhost:9999"}
	hasher := hasher.NewHasher(loggregatorServers)
	r, err := NewRouter("localhost:3456", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go r.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3456", "ROUTER", "42", logger)
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
	hasher := hasher.NewHasher(loggregatorServers)
	r, err := NewRouter("localhost:3455", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go r.Start(logger)
	time.Sleep(50 * time.Millisecond)

	lc := loggregatorclient.NewLoggregatorClient("localhost:3455", logger, loggregatorclient.DefaultBufferSize)
	lc.Send([]byte("This is poorly formatted"))

	logEmitter, _ := emitter.NewEmitter("localhost:3455", "ROUTER", "42", logger)
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
	hasher := hasher.NewHasher(loggregatorServers)
	rt, err := NewRouter("localhost:3457", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go rt.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3457", "ROUTER", "42", logger)
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
