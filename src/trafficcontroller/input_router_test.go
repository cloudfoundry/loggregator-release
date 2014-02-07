package trafficcontroller

import (
	"code.google.com/p/gogoprotobuf/proto"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"trafficcontroller/hasher"
)

var logger = steno.NewLogger("TestLogger")

func newCfConfig() cfcomponent.Config {
	return cfcomponent.Config{}
}

func TestThatItWorksWithOneLoggregator(t *testing.T) {
	listener, dataChannel := agentlistener.NewAgentListener("localhost:9999", logger)
	go listener.Start()

	loggregatorServers := []string{"localhost:9999"}
	hasher := hasher.NewHasher(loggregatorServers)
	r, err := NewRouter("localhost:3456", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go r.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3456", "ROUTER", "42", "secret", logger)
	logEmitter.Emit("my_awesome_app", "Hello World")

	received := <-dataChannel

	receivedEnvelope := &logmessage.LogEnvelope{}
	proto.Unmarshal(received, receivedEnvelope)

	assert.Equal(t, receivedEnvelope.GetLogMessage().GetAppId(), "my_awesome_app")
	assert.Equal(t, string(receivedEnvelope.GetLogMessage().GetMessage()), "Hello World")
}

func TestThatItIgnoresBadMessages(t *testing.T) {
	listener, dataChannel := agentlistener.NewAgentListener("localhost:9996", logger)
	go listener.Start()

	loggregatorServers := []string{"localhost:9996"}
	hasher := hasher.NewHasher(loggregatorServers)
	r, err := NewRouter("localhost:3455", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go r.Start(logger)
	time.Sleep(50 * time.Millisecond)

	lc := loggregatorclient.NewLoggregatorClient("localhost:3455", logger, loggregatorclient.DefaultBufferSize)
	lc.Send([]byte("This is poorly formatted"))

	logEmitter, _ := emitter.NewEmitter("localhost:3455", "ROUTER", "42", "secret", logger)
	logEmitter.Emit("my_awesome_app", "Hello World")

	received := <-dataChannel
	receivedEnvelope := &logmessage.LogEnvelope{}
	proto.Unmarshal(received, receivedEnvelope)

	assert.Equal(t, receivedEnvelope.GetLogMessage().GetAppId(), "my_awesome_app")
	assert.Equal(t, string(receivedEnvelope.GetLogMessage().GetMessage()), "Hello World")
}

func TestThatItWorksWithTwoLoggregators(t *testing.T) {
	listener1, dataChan1 := agentlistener.NewAgentListener("localhost:9998", logger)
	go listener1.Start()

	listener2, dataChan2 := agentlistener.NewAgentListener("localhost:9997", logger)
	go listener2.Start()

	loggregatorServers := []string{"localhost:9998", "localhost:9997"}
	hasher := hasher.NewHasher(loggregatorServers)
	rt, err := NewRouter("localhost:3457", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go rt.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3457", "ROUTER", "42", "secret", logger)
	logEmitter.Emit("2", "My message")

	receivedData := <-dataChan1
	receivedEnvelope := &logmessage.LogEnvelope{}
	proto.Unmarshal(receivedData, receivedEnvelope)

	assert.Equal(t, receivedEnvelope.GetLogMessage().GetAppId(), "2")
	assert.Equal(t, string(receivedEnvelope.GetLogMessage().GetMessage()), "My message")

	logEmitter.Emit("1", "Another message")

	receivedData = <-dataChan2
	receivedEnvelope = &logmessage.LogEnvelope{}
	proto.Unmarshal(receivedData, receivedEnvelope)

	assert.Equal(t, receivedEnvelope.GetLogMessage().GetAppId(), "1")
	assert.Equal(t, string(receivedEnvelope.GetLogMessage().GetMessage()), "Another message")
}

func TestThatItWorksWithLogEnvelope(t *testing.T) {
	listener, dataChannel := agentlistener.NewAgentListener("localhost:9902", logger)
	go listener.Start()

	loggregatorServers := []string{"localhost:9902"}
	hasher := hasher.NewHasher(loggregatorServers)
	r, err := NewRouter("localhost:3551", hasher, newCfConfig(), logger)
	assert.NoError(t, err)

	go r.Start(logger)
	time.Sleep(50 * time.Millisecond)

	logEmitter, _ := emitter.NewEmitter("localhost:3551", "RTR", "42", "secret", logger)
	logEmitter.Emit("my_awesome_app", "Hello World")

	received := <-dataChannel

	receivedEnvelope := &logmessage.LogEnvelope{}
	proto.Unmarshal(received, receivedEnvelope)

	assert.Equal(t, receivedEnvelope.GetLogMessage().GetAppId(), "my_awesome_app")
	assert.Equal(t, string(receivedEnvelope.GetLogMessage().GetMessage()), "Hello World")
}
