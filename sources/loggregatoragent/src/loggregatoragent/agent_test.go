package loggregatoragent

import (
	"cfcomponent/instrumentation"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"logmessage"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type MockLoggregatorClient struct {
	received chan *[]byte
}

func (m MockLoggregatorClient) Send(data []byte) {
	m.received <- &data
}

func (m MockLoggregatorClient) Emit() instrumentation.Context {
	return instrumentation.Context{}
}

func (m MockLoggregatorClient) IncLogStreamRawByteCount(uint64) {

}

func (m MockLoggregatorClient) IncLogStreamPbByteCount(uint64) {

}

func getBackendMessage(t *testing.T, data *[]byte) *logmessage.LogMessage {
	receivedMessage := &logmessage.LogMessage{}

	err := proto.Unmarshal(*data, receivedMessage)

	if err != nil {
		t.Fatalf("Message invalid. %s", err)
	}
	return receivedMessage
}

func makeBackendMessage(t *testing.T, messageString string) []byte {
	currentTime := time.Now()
	sourceType := logmessage.LogMessage_CLOUD_CONTROLLER
	messageType := logmessage.LogMessage_OUT
	message := &logmessage.LogMessage{
		Message:        []byte(messageString),
		AppId:          proto.String("myApp"),
		SpaceId:        proto.String("mySpace"),
		OrganizationId: proto.String("myOrg"),
		MessageType:    &messageType,
		SourceType:     &sourceType,
		SourceId:       proto.String("sourceId"),
		Timestamp:      proto.Int64(currentTime.UnixNano()),
	}

	data, err := proto.Marshal(message)

	if err != nil {
		t.Fatalf("Message invalid. %s", err)
	}
	return data
}

func TestListensToUnixSocket(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Fatal("unixgrams don't work on darwin... run tests on linux")
	}

	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	socketPath := filepath.Join(tmpdir, "component.sock")
	os.Remove(socketPath)

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)
	agent := NewAgent(socketPath, gosteno.NewLogger("TestLogger"))
	go agent.Start(*mockLoggregatorClient)

	runtime.Gosched()

	expectedMessage := "Some Output\n"
	messageBytes := makeBackendMessage(t, expectedMessage)

	addr, err := net.ResolveUnixAddr("unixgram", socketPath)
	assert.NoError(t, err)
	connection, err := net.DialUnix("unixgram", nil, addr)
	assert.NoError(t, err)

	_, err = connection.WriteToUnix(messageBytes, addr)
	assert.NoError(t, err)

	receivedBytes := <-mockLoggregatorClient.received
	receivedMessage := getBackendMessage(t, receivedBytes)

	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}
