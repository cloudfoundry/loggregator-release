package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	testhelpers "server_testhelpers"
	"testing"
)

var TestRsyslogServer net.TCPListener
var dataReadChannel chan []byte

func init() {
	dataReadChannel = make(chan []byte, 10)
	testSink, err := net.Listen("tcp", "localhost:24631")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			buffer := make([]byte, 1024)
			conn, err := testSink.Accept()
			defer conn.Close()
			if err != nil {
				panic(err)
			}
			readCount, err := conn.Read(buffer)
			if err != nil {
				panic(err)
			}
			dataReadChannel <- buffer[:readCount]
		}
	}()
	runtime.Gosched()
}

func TestThatItSendsStdOutAsInfo(t *testing.T) {
	sink, err := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger())
	assert.NoError(t, err)
	closeChan := make(chan chan *logmessage.Message)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(testhelpers.MarshalledLogMessage(t, "hi", "appId"))
	assert.NoError(t, err)
	sink.ListenerChannel() <- logMessage
	data := <-dataReadChannel
	assert.Contains(t, string(data), "<6>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "hi")
}

func TestThatItSendsStdErrAsErr(t *testing.T) {
	sink, err := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger())
	assert.NoError(t, err)
	closeChan := make(chan chan *logmessage.Message)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(testhelpers.MarshalledErrorLogMessage(t, "err", "appId"))
	assert.NoError(t, err)
	sink.ListenerChannel() <- logMessage
	data := <-dataReadChannel
	assert.Contains(t, string(data), "<3>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "err")
}
