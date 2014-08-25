package sinkserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
)

func NewMessage(messageString, appId string) *envelopewrapper.WrappedEnvelope {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, messageString, appId, "App")
	env, _ := emitter.Wrap(logMessage, "origin")
	envBytes, _ := proto.Marshal(env)
	wrappedEnv := &envelopewrapper.WrappedEnvelope{
		Envelope:      env,
		EnvelopeBytes: envBytes,
	}

	return wrappedEnv
}

func NewLogMessage(messageString, appId string) *events.LogMessage {
	messageType := events.LogMessage_OUT
	sourceType := "App"

	return factories.NewLogMessage(messageType, messageString, appId, sourceType)
}

func TestSinkserver(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {

	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)
	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})

	if err != nil {
		Fail(err.Error())
	}
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
