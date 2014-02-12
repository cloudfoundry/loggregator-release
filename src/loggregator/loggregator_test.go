package loggregator_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"net/http"
	"time"
)

// TODO: test error logging and metrics from unmarshaller stage
// messageRouter.Metrics.UnmarshalErrorsInParseEnvelopes++
//messageRouter.logger.Errorf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, envelopedLog)

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	var ws *websocket.Conn
	i := 0
	for {
		var err error
		ws, _, err = websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
		if err != nil {
			i++
			if i > 10 {
				fmt.Printf("Unable to connect to Server in 100ms, giving up.\n")
				return nil, nil, nil
			}
			<-time.After(10 * time.Millisecond)
			continue
		} else {
			break
		}

	}

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				close(connectionDroppedChannel)
				close(receivedChan)
				return
			}
			receivedChan <- data
		}

	}()
	go func() {
		for {
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{42})
			if err != nil {
				break
			}
			select {
			case <-dontKeepAliveChan:
				return
			case <-time.After(4 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}

func UnmarshallerMaker(secret string) func([]byte) (*logmessage.Message, error) {
	return func(data []byte) (*logmessage.Message, error) {
		return logmessage.ParseEnvelope(data, secret)
	}
}

func MarshalledLogEnvelope(unmarshalledMessage *logmessage.LogMessage, secret string) []byte {
	envelope := &logmessage.LogEnvelope{
		LogMessage: unmarshalledMessage,
		RoutingKey: proto.String(*unmarshalledMessage.AppId),
	}
	envelope.SignEnvelope(secret)

	return marshalProtoBuf(envelope)
}

func marshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	if err != nil {
		Fail(err.Error())
	}

	return marshalledProtoBuf
}

func parseProtoBufMessageString(actual []byte) string {
	receivedMessage := &logmessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	if err != nil {
		Fail(err.Error())
	}
	return string(receivedMessage.GetMessage())
}

var _ = Describe("Loggregator Server", func() {

	var receivedChan chan []byte
	var dontKeepAliveChan chan bool
	var connectionDroppedChannel <-chan bool
	var ws *websocket.Conn

	BeforeEach(func() {
		receivedChan = make(chan []byte)
		ws, dontKeepAliveChan, connectionDroppedChannel = AddWSSink(receivedChan, "8083", "/tail/?app=myApp")
		if ws == nil || dontKeepAliveChan == nil || connectionDroppedChannel == nil {

		}

	})

	AfterEach(func(done Done) {
		if dontKeepAliveChan != nil {
			close(dontKeepAliveChan)
			ws.Close()
			Eventually(receivedChan).Should(BeClosed())
			close(done)
		}
	})

	It("should work from udp socket to websocket client", func() {

		connection, err := net.Dial("udp", "127.0.0.1:3456")
		Expect(err).To(BeNil())

		expectedMessageString := "Some Data"
		unmarshalledLogMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")

		expectedMessage := MarshalledLogEnvelope(unmarshalledLogMessage, "secret")

		_, err = connection.Write(expectedMessage)
		Expect(err).To(BeNil())

		receivedMessageString := parseProtoBufMessageString(<-receivedChan)
		Expect(expectedMessageString).To(Equal(receivedMessageString))
	})

	It("should drop invalid log envelopes", func() {
		time.Sleep(50 * time.Millisecond)

		connection, err := net.Dial("udp", "127.0.0.1:3456")
		Expect(err).To(BeNil())

		expectedMessageString := "Some Data"
		unmarshalledLogMessage := messagetesthelpers.NewLogMessage(expectedMessageString, "myApp")

		expectedMessage := MarshalledLogEnvelope(unmarshalledLogMessage, "invalid")

		_, err = connection.Write(expectedMessage)
		Expect(err).To(BeNil())
		Expect(receivedChan).To(BeEmpty())
	})
})
