package sinkserver_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/gorilla/websocket"
	"net/http"
	testhelpers "server_testhelpers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dumping", func() {
	It("dumps all messages for an app user", func() {
		expectedMessageString := "Some data"

		lm := factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myOtherApp", "APP")
		env, _ := emitter.Wrap(lm, "ORIGIN")
		envBytes, _ := proto.Marshal(env)
		wrappedEnvelope := &envelopewrapper.WrappedEnvelope{
			Envelope:      env,
			EnvelopeBytes: envBytes,
		}

		dataReadChannel <- wrappedEnvelope
		dataReadChannel <- wrappedEnvelope

		receivedChan := make(chan []byte, 2)
		_, stopKeepAlive, droppedChannel := testhelpers.AddWSSink(GinkgoT(), receivedChan, SERVER_PORT, "/dump/?app=myOtherApp")

		select {
		case <-droppedChannel:
			// we should have been dropped
		case <-time.After(10 * time.Millisecond):
			Fail("we should have been dropped")
		}

		logMessages := dumpAllMessages(receivedChan)

		Expect(logMessages).To(HaveLen(2))
		marshalledEnvelope := logMessages[1]
		var envelope events.Envelope
		proto.Unmarshal(marshalledEnvelope, &envelope)
		Expect(envelope.GetLogMessage().GetMessage()).To(BeEquivalentTo(expectedMessageString))

		stopKeepAlive <- true
	})

	It("doesn't hang when there are no messages", func() {
		receivedChan := make(chan []byte, 1)
		testhelpers.AddWSSink(GinkgoT(), receivedChan, SERVER_PORT, "/dump/?app=myOtherApp")

		doneChan := make(chan bool)
		go func() {
			dumpAllMessages(receivedChan)
			close(doneChan)
		}()
		select {
		case <-doneChan:
			break
		case <-time.After(10 * time.Millisecond):
			Fail("Should have returned by now")
		}

	})

	It("errors when log target is invalid", func() {
		path := "/dump/?something=invalidtarget"
		_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+SERVER_PORT+path, http.Header{})
		Expect(err).To(HaveOccurred())
	})
})

func dumpAllMessages(receivedChan chan []byte) [][]byte {
	logMessages := [][]byte{}
	for message := range receivedChan {
		logMessages = append(logMessages, message)
	}
	return logMessages
}
