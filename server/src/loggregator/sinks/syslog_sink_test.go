package sinks

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type SyslogWriterRecorder struct {
	receivedChannel  chan string
	receivedMessages []string
	up               bool
	connected        bool
	*sync.Mutex
}

func NewSyslogWriterRecorder() *SyslogWriterRecorder {
	return &SyslogWriterRecorder{
		make(chan string, 20),
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

func (r *SyslogWriterRecorder) WriteStdout(b []byte, source string, timestamp int64) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.up {
		r.receivedMessages = append(r.receivedMessages, "out: "+string(b))
		r.receivedChannel <- string(b)
		return len(b), nil
	} else {
		return 0, errors.New("Error writing to stdout.")
	}
}

func (r *SyslogWriterRecorder) WriteStderr(b []byte, source string, timestamp int64) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.up {
		r.receivedMessages = append(r.receivedMessages, "err: "+string(b))
		r.receivedChannel <- string(b)
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
	sink := NewSyslogSink("appId", "syslog://localhost:24631", loggertesthelper.Logger(), sysLogger)

	go sink.Run()
	defer close(sink.Channel())

	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledLogMessage(t, "hi", "appId"), "")
	assert.NoError(t, err)
	sink.Channel() <- logMessage
	data := <-fakeSyslogServer.dataReadChannel
	assert.Contains(t, string(data), "61 <14>1 ")
	assert.Contains(t, string(data), " loggregator appId DEA - - hi\n")
}

func TestThatItStripsNullControlCharacterFromMsg(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24631", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24631", loggertesthelper.Logger(), sysLogger)

	go sink.Run()
	defer close(sink.Channel())

	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledLogMessage(t, string(0)+" hi", "appId"), "")
	assert.NoError(t, err)
	sink.Channel() <- logMessage

	data := <-fakeSyslogServer.dataReadChannel
	assert.NotContains(t, string(data), "\000")
	assert.Contains(t, string(data), "<14>1")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "hi")
}

func TestThatItSendsStdErrAsErr(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24632", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24632", loggertesthelper.Logger(), sysLogger)
	go sink.Run()
	defer close(sink.Channel())

	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"), "")
	assert.NoError(t, err)

	sink.Channel() <- logMessage
	data := <-fakeSyslogServer2.dataReadChannel

	assert.Contains(t, string(data), "<11>1")
	assert.Contains(t, string(data), "appId")
	assert.Contains(t, string(data), "err")
}

func TestThatItUsesOctetFramingWhenSending(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24632", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24632", loggertesthelper.Logger(), sysLogger)
	go sink.Run()
	defer close(sink.Channel())

	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"), "")
	assert.NoError(t, err)

	sink.Channel() <- logMessage
	data := <-fakeSyslogServer2.dataReadChannel

	syslogMsg := string(data)

	syslogRegexp := regexp.MustCompile(`<11>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}([-+]\d{2}:\d{2}) loggregator appId DEA - - err\n`)
	msgBeforeOctetCounting := syslogRegexp.FindString(syslogMsg)
	assert.True(t, strings.HasPrefix(syslogMsg, strconv.Itoa(len(msgBeforeOctetCounting))+" "))
}

func TestThatItUsesTheOriginalTimestampOfTheLogmessageWhenSending(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:24632", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:24632", loggertesthelper.Logger(), sysLogger)
	go sink.Run()
	defer close(sink.Channel())

	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"), "")
	expectedTimeString := strings.Replace(time.Unix(0, logMessage.GetLogMessage().GetTimestamp()).Format(time.RFC3339), "Z", "+00:00", 1)

	assert.NoError(t, err)

	time.Sleep(1200 * time.Millisecond) //wait so that a second will pass, to allow timestamps to differ

	sink.Channel() <- logMessage
	data := <-fakeSyslogServer2.dataReadChannel

	assert.Equal(t, "62 <11>1 "+expectedTimeString+" loggregator appId DEA - - err\n", string(data))
}

func TestThatItHandlesMessagesEvenIfThereIsNoSyslogServer(t *testing.T) {
	sysLogger := NewSyslogWriter("tcp", "localhost:-1", "appId")
	sink := NewSyslogSink("appId", "syslog://localhost:-1", loggertesthelper.Logger(), sysLogger)
	go sink.Run()
	defer close(sink.Channel())
	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, "err", "appId"), "")
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		sink.Channel() <- logMessage
	}
}

func TestSysLoggerComesUpLate(t *testing.T) {
	sysLogger := NewSyslogWriterRecorder()
	sysLogger.SetUp(false)
	sink := NewSyslogSink("appId", "url_not_used", loggertesthelper.Logger(), sysLogger)

	done := make(chan bool)
	go func() {
		sink.Run()
		done <- true
	}()

	for i := 0; i < 15; i++ {
		msg := fmt.Sprintf("message no %v", i)
		logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, msg, "appId"), "")
		assert.NoError(t, err)

		sink.Channel() <- logMessage
	}
	close(sink.Channel())

	sysLogger.SetUp(true)
	<-done

	data := sysLogger.ReceivedMessages()
	assert.Equal(t, len(data), 10)

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("err: message no %v", i+5)
		assert.Equal(t, data[i], msg)
	}
}

func TestSysLoggerDiesAndComesBack(t *testing.T) {
	sysLogger := NewSyslogWriterRecorder()
	sink := NewSyslogSink("appId", "url_not_used", loggertesthelper.Logger(), sysLogger)
	sysLogger.SetUp(true)

	done := make(chan bool)
	go func() {
		sink.Run()
		done <- true
	}()

	msg := fmt.Sprintf("first message")
	logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, msg, "appId"), "")
	assert.NoError(t, err)
	sink.Channel() <- logMessage

	select {
	case <-sysLogger.receivedChannel:
		break
	case <-time.After(10 * time.Millisecond):
		t.Error("Should have received a message by now")
	}
	sysLogger.SetUp(false)

	assert.Equal(t, len(sysLogger.ReceivedMessages()), 1)

	for i := 0; i < 11; i++ {
		msg := fmt.Sprintf("message no %v", i)
		logMessage, err := logmessage.ParseProtobuffer(messagetesthelpers.MarshalledErrorLogMessage(t, msg, "appId"), "")
		assert.NoError(t, err)

		sink.Channel() <- logMessage
	}

	sysLogger.SetUp(true)
	close(sink.Channel())
	<-done

	assert.Equal(t, len(sysLogger.ReceivedMessages()), 11)

	stringOfMessages := fmt.Sprintf("%v", sysLogger.ReceivedMessages())
	assert.Contains(t, stringOfMessages, "first message", "This message should have been there, since the server was up")
	assert.NotContains(t, stringOfMessages, "err: message no 0", "This message should have been lost because the connection problem was detected while trying to send it.")
	assert.Contains(t, stringOfMessages, "err: message no 1", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 2", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 3", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 4", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 5", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 6", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 7", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 8", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 9", "This message should have been there, since it was in the ringbuffer while the server was down")
	assert.Contains(t, stringOfMessages, "err: message no 10", "This message should have been there, since it was in the ringbuffer while the server was down")
}

var backoffTests = []struct {
	backoffCount int
	expected     time.Duration
}{
	{1, 1000},
	{2, 2000},
	{3, 4000},
	{4, 8000},
	{5, 16000},
	{11, 1024000},    //1.024s
	{12, 2048000},    //2.048s
	{20, 524288000},  //8m and a bit
	{21, 1048576000}, //17m28.576s
	{22, 2097152000}, //34m57.152s
	{23, 4194304000}, //1h9m54.304s
	{24, 4194304000}, //1h9m54.304s
	{25, 4194304000}, //1h9m54.304s
	{26, 4194304000}, //1h9m54.304s
}

func TestExponentialRetryStrategy(t *testing.T) {
	rand.Seed(1)
	strategy := newExponentialRetryStrategy()
	otherStrategy := newExponentialRetryStrategy()

	assert.Equal(t, strategy(0).String(), "0")
	assert.Equal(t, otherStrategy(0).String(), "0")

	var backoff time.Time
	var otherBackoff time.Time

	now := time.Now()
	oldBackoff := time.Now()
	otherOldBackoff := time.Now()

	for _, bt := range backoffTests {
		delta := int(bt.expected / 10)
		for i := 0; i < 10; i++ {
			backoff = now.Add(strategy(bt.backoffCount))
			otherBackoff = now.Add(otherStrategy(bt.backoffCount))

			assert.NotEqual(t, backoff, otherBackoff)
			assert.WithinDuration(t,
				now.Add(time.Duration(bt.expected)*time.Microsecond),
				backoff,
				time.Duration(delta)*time.Microsecond)
			assert.WithinDuration(t,
				now.Add(time.Duration(bt.expected)*time.Microsecond),
				otherBackoff,
				time.Duration(delta)*time.Microsecond)
			assert.NotEqual(t, oldBackoff, backoff)
			assert.NotEqual(t, otherOldBackoff, otherBackoff)

			oldBackoff = backoff
			otherOldBackoff = otherBackoff
		}
	}
}
