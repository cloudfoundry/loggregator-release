package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	testhelpers "server_testhelpers"
	"testing"
)

var TestRsyslogServer net.TCPListener
var dataReadChannel chan []byte
var closeChannel chan bool

func init() {
	dataReadChannel = make(chan []byte, 10)
	closeChannel = make(chan bool)
	testSink, err := net.Listen("tcp", "localhost:24631")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			buffer := make([]byte, 1024)
			conn, err := testSink.Accept()
			defer conn.Close()
			defer testSink.Close()
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
	sink := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger())

	closeChan := make(chan Sink)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledLogMessage(t, "hi", "appId"))
	assert.NoError(t, err)
	sink.Channel() <- logMessage
	data := <-dataReadChannel
	assert.Contains(t, string(data), "<6>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "hi")
}

func TestThatItSendsStdErrAsErr(t *testing.T) {
	sink := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger())
	closeChan := make(chan Sink)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"))
	assert.NoError(t, err)
	sink.Channel() <- logMessage
	data := <-dataReadChannel
	assert.Contains(t, string(data), "<3>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "err")
}

func TestThatItClosesWhenNoConnectionIsEstablished(t *testing.T) {
	sink := NewSyslogSink("appId", "syslog://badsyslog:24632", testhelpers.Logger())
	closeChan := make(chan Sink)

	go func() {
		assert.NotPanics(t, func() { sink.Run(closeChan) })
	}()

	<-closeChan
}
