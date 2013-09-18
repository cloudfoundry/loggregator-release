package sinks

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"net"
	testhelpers "server_testhelpers"
	"sync"
	"testing"
	"time"
)

type SyslogWriterRecorder struct {
	receivedMessages []string
	up               bool
	connected        bool
	*sync.Mutex
}

func NewSyslogWriterRecorder() *SyslogWriterRecorder {
	return &SyslogWriterRecorder{
		[]string{},
		false,
		false,
		&sync.Mutex{},
	}
}

func (r *SyslogWriterRecorder) Connect() error {
	r.Lock()
	defer r.Unlock()

	if r.up {
		r.connected = true
		return nil
	} else {
		r.connected = false
		return errors.New("Error connecting.")
	}
}

func (r *SyslogWriterRecorder) WriteStdout(b []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.up {
		r.receivedMessages = append(r.receivedMessages, "out: "+string(b))
		return len(b), nil
	} else {
		return 0, errors.New("Error writing to stdout.")
	}
}

func (r *SyslogWriterRecorder) WriteStderr(b []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.up {
		r.receivedMessages = append(r.receivedMessages, "err: "+string(b))
		return len(b), nil
	} else {
		return 0, errors.New("Error writing to stderr.")
	}
}

func (r *SyslogWriterRecorder) SetUp(newState bool) {
	r.Lock()
	defer r.Unlock()

	r.up = newState
}

func (w *SyslogWriterRecorder) IsConnected() bool {
	return w.connected
}

func (w *SyslogWriterRecorder) SetConnected(newValue bool) {
	w.connected = newValue
}

func (r *SyslogWriterRecorder) Close() error {
	return nil
}

func (r *SyslogWriterRecorder) ReceivedMessages() []string {
	r.Lock()
	defer r.Unlock()

	return r.receivedMessages
}

var fakeSyslogServer FakeSyslogServer
var fakeSyslogServer2 FakeSyslogServer

type FakeSyslogServer struct {
	dataReadChannel chan []byte
	closeChannel    chan bool
}

func startFakeSyslogServer(port string) FakeSyslogServer {
	syslogServer := &FakeSyslogServer{make(chan []byte, 10), make(chan bool)}
	testSink, err := net.Listen("tcp", "localhost:"+port)
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
				continue
			}

			syslogServer.dataReadChannel <- buffer[:readCount]
		}
	}()
	time.Sleep(10 * time.Millisecond)
	return *syslogServer
}

func init() {
	fakeSyslogServer = startFakeSyslogServer("24631")
	fakeSyslogServer2 = startFakeSyslogServer("24632")
}

func TestThatItSendsStdOutAsInfo(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24631", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger(), sysLogger)

	closeChan := make(chan Sink)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledLogMessage(t, "hi", "appId"))
	assert.NoError(t, err)
	sink.Channel() <- logMessage
	data := <-fakeSyslogServer.dataReadChannel
	assert.Contains(t, string(data), "<6>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "hi")
}

func TestThatItSendsStdErrAsErr(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24632", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24632", testhelpers.Logger(), sysLogger)
	closeChan := make(chan Sink)
	go sink.Run(closeChan)

	logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"))
	assert.NoError(t, err)

	sink.Channel() <- logMessage
	data := <-fakeSyslogServer2.dataReadChannel

	assert.Contains(t, string(data), "<3>")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "err")
}

func TestThatItHandlesMessagesEvenIfThereIsNoSyslogServer(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24631", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger(), sysLogger)
	closeChan := make(chan Sink)
	go sink.Run(closeChan)
	logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"))
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		sink.Channel() <- logMessage
	}
}

func TestSysLoggerComesUpLate(t *testing.T) {
	sysLogger := NewSyslogWriterRecorder()
	sink := NewSyslogSink("appId", "syslog://localhost:24631", testhelpers.Logger(), sysLogger)

	closeChan := make(chan Sink)
	go sink.Run(closeChan)

	go func() {
		for i := 0; i < 15; i++ {
			time.Sleep(1 * time.Millisecond)

			msg := fmt.Sprintf("message no %v", i)
			logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledErrorLogMessage(t, msg, "appId"))
			assert.NoError(t, err)

			sink.Channel() <- logMessage
		}
	}()
	time.Sleep(30 * time.Millisecond)

	sysLogger.SetUp(true)
	time.Sleep(100 * time.Millisecond)

	data := sysLogger.ReceivedMessages()
	assert.Equal(t, len(data), 10)

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("err: message no %v", i+5)
		assert.Equal(t, data[i], msg)
	}
}

func TestSysLoggerDiesAndComesBack(t *testing.T) {
	sysLogger := NewSyslogWriterRecorder()
	sink := NewSyslogSink("appId", "syslog://localhost:24633", testhelpers.Logger(), sysLogger)
	sysLogger.SetUp(true)

	closeChan := make(chan Sink)
	go sink.Run(closeChan)

	go func() {
		for i := 0; i < 10; i++ {
			msg := fmt.Sprintf("message no %v", i)
			logMessage, err := logmessage.ParseMessage(messagetesthelpers.MarshalledErrorLogMessage(t, msg, "appId"))
			assert.NoError(t, err)

			sink.Channel() <- logMessage
			time.Sleep(100 * time.Millisecond)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	sysLogger.SetUp(false)
	assert.Equal(t, len(sysLogger.ReceivedMessages()), 1)

	time.Sleep(800 * time.Millisecond)
	sysLogger.SetUp(true)
	time.Sleep(300 * time.Millisecond)

	assert.Equal(t, len(sysLogger.ReceivedMessages()), 9)

	stringOfMessages := fmt.Sprintf("%v", sysLogger.ReceivedMessages())
	assert.Contains(t, stringOfMessages, "err: message no 0", "This message should have been there, since the server was up")
	assert.NotContains(t, stringOfMessages, "err: message no 1", "This message should have been lost because the connection problem was detected while trying to send it.")
	assert.Contains(t, stringOfMessages, "err: message no 2", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 3", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 4", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 5", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 6", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 7", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 8", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 9", "This message should have been there, since it was in the ringbuffer while the server was down")

}

var backoffTests = []struct {
	backoffCount int
	expected     time.Duration
}{
	{1, 1000},
	{5, 16000},
	{11, 1024000},    //1.024s
	{20, 524288000},  //8m and a bit
	{21, 1048576000}, //17m28.576s
	{22, 2097152000}, //34m57.152s
	{23, 4194304000}, //1h9m54.304s
	{24, 4194304000}, //1h9m54.304s
	{25, 4194304000}, //1h9m54.304s
	{26, 4194304000}, //1h9m54.304s
}

func TestExponentialRetryStrategy(t *testing.T) {
	strategy := newExponentialRetryStrategy()

	assert.Equal(t, strategy(0).String(), "0")

	var backoff time.Time
	now := time.Now()
	oldBackoff := time.Now()

	for _, bt := range backoffTests {
		delta := int(bt.expected / 10)
		for i := 0; i < 10; i++ {
			backoff = now.Add(strategy(bt.backoffCount))
			assert.WithinDuration(t,
				now.Add(time.Duration(bt.expected)*time.Microsecond),
				backoff,
				time.Duration(delta)*time.Microsecond)
			assert.NotEqual(t, oldBackoff, backoff)
			oldBackoff = backoff
		}
	}
}
