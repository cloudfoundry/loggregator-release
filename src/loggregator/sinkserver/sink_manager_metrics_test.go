package sinkserver

import (
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	oldDumpSinksCounter := sinkManager.Emit().Metrics[0].Value.(int)
	oldSyslogSinksCounter := sinkManager.Emit().Metrics[1].Value.(int)
	oldWebsocketSinksCounter := sinkManager.Emit().Metrics[2].Value.(int)

	clientReceivedChan := make(chan []byte)
	fakeSyslogDrain, err := NewFakeService(clientReceivedChan, "127.0.0.1:32564")
	assert.NoError(t, err)
	go fakeSyslogDrain.Serve()
	<-fakeSyslogDrain.ReadyChan

	assert.Equal(t, sinkManager.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[0].Value, oldDumpSinksCounter)

	assert.Equal(t, sinkManager.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[1].Value, oldSyslogSinksCounter)

	assert.Equal(t, sinkManager.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	logEnvelope := messagetesthelpers.MarshalledLogEnvelopeForMessage(t, "expectedMessageString", "myMetricsApp", SECRET, "syslog://localhost:32564")
	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-clientReceivedChan:
	}

	assert.Equal(t, sinkManager.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	dataReadChannel <- logEnvelope

	select {
	case <-time.After(1000 * time.Millisecond):
		t.Errorf("Did not get message 1")
	case <-clientReceivedChan:
	}

	assert.Equal(t, sinkManager.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[2].Value, oldWebsocketSinksCounter)

	receivedChan := make(chan []byte, 2)

	_, dontKeepAliveChan, _ := testhelpers.AddWSSink(t, receivedChan, SERVER_PORT, TAIL_PATH+"?app=myMetricsApp")
	WaitForWebsocketRegistration()

	assert.Equal(t, sinkManager.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[2].Value, oldWebsocketSinksCounter+1)

	dontKeepAliveChan <- true
	WaitForWebsocketRegistration()

	assert.Equal(t, sinkManager.Emit().Metrics[0].Name, "numberOfDumpSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[0].Value, oldDumpSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[1].Name, "numberOfSyslogSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[1].Value, oldSyslogSinksCounter+1)

	assert.Equal(t, sinkManager.Emit().Metrics[2].Name, "numberOfWebsocketSinks")
	assert.Equal(t, sinkManager.Emit().Metrics[2].Value, oldWebsocketSinksCounter)
}
