package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregator/iprange"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

var TestMessageRouter *messageRouter
var TestHttpServer *httpServer
var dataReadChannel chan []byte

var blacklistTestMessageRouter *messageRouter
var blackListTestHttpServer *httpServer
var blackListDataReadChannel chan []byte

const (
	SERVER_PORT           = "8081"
	BLACKLIST_SERVER_PORT = "8082"
)

const SECRET = "secret"

func init() {
	dataReadChannel = make(chan []byte, 20)
	TestMessageRouter = NewMessageRouter(1024, false, nil, loggertesthelper.Logger())
	go TestMessageRouter.Start()
	TestHttpServer = NewHttpServer(TestMessageRouter, 10*time.Millisecond, testhelpers.UnmarshallerMaker(SECRET), 100, loggertesthelper.Logger())
	go TestHttpServer.Start(dataReadChannel, "localhost:"+SERVER_PORT)

	blackListDataReadChannel = make(chan []byte, 20)
	blacklistTestMessageRouter = NewMessageRouter(1024, false, []iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.2"}}, loggertesthelper.Logger())
	go blacklistTestMessageRouter.Start()
	blackListTestHttpServer = NewHttpServer(blacklistTestMessageRouter, 10*time.Millisecond, testhelpers.UnmarshallerMaker(SECRET), 100, loggertesthelper.Logger())
	go blackListTestHttpServer.Start(blackListDataReadChannel, "localhost:"+BLACKLIST_SERVER_PORT)
	time.Sleep(2 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnectionFails(t *testing.T, port string, path string, expectedErrorCode uint16) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)

	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)
	data := make([]byte, 2)
	_, err = ws.Read(data)
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}

func TestMetrics(t *testing.T) {
	oldDumpSinksCounter := TestMessageRouter.Emit().Metrics[0].Value.(int)
	oldSyslogSinksCounter := TestMessageRouter.Emit().Metrics[1].Value.(int)
	oldWebsocketSinksCounter := TestMessageRouter.Emit().Metrics[2].Value.(int)

	clientReceivedChan := make(chan []byte)
	fakeSyslogDrain, err := NewFakeService(clientReceivedChan, "127.0.0.1:32564")
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, "expectedMessageString", "myMetricsApp", SECRET, "syslog://localhost:32564")
	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-clientReceivedChan:
	}

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-clientReceivedChan:
	}

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	receivedChan := make(chan []byte, 2)

	_, dontKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myMetricsApp")
	WaitForWebsocketRegistration()

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter+1)

	dontKeepAliveChan <- true
	WaitForWebsocketRegistration()

	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, TestMessageRouter.Emit().Metrics[2].Value, oldWebsocketSinksCounter)
}
