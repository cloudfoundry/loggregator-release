package endtoend

import (
	"crypto/tls"
	"strings"
	"sync"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/pivotal-golang/localip"
)

type FirehoseReader struct {
	consumer *consumer.Consumer
	msgChan  <-chan *events.Envelope
	stopChan chan struct{}

	TestMetricCount             float64
	NonTestMetricCount          float64
	MetronSentMessageCount      float64
	DopplerReceivedMessageCount float64
	DopplerSentMessageCount     float64

	logMessageMu   sync.Mutex
	lastLogMessage *events.Envelope
}

func NewFirehoseReader() *FirehoseReader {
	consumer, msgChan := initiateFirehoseConnection()
	return &FirehoseReader{
		consumer: consumer,
		msgChan:  msgChan,
		stopChan: make(chan struct{}),

		TestMetricCount:             0,
		NonTestMetricCount:          0,
		MetronSentMessageCount:      0,
		DopplerReceivedMessageCount: 0,
	}
}

func (r *FirehoseReader) Read() {
	select {
	case <-r.stopChan:
		return
	case msg := <-r.msgChan:
		if testMetric(msg) {
			r.TestMetricCount += 1
		} else {
			r.NonTestMetricCount += 1
		}

		if metronSentMessageCount(msg) {
			r.MetronSentMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if dopplerReceivedMessageCount(msg) {
			r.DopplerReceivedMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if dopplerSentMessageCount(msg) {
			r.DopplerSentMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if isLogMessage(msg) {
			r.logMessageMu.Lock()
			r.lastLogMessage = msg
			r.logMessageMu.Unlock()
		}
	}
}

func (r *FirehoseReader) Close() {
	close(r.stopChan)
	r.consumer.Close()
}

func (r *FirehoseReader) LastLogMessage() *events.Envelope {
	r.logMessageMu.Lock()
	defer r.logMessageMu.Unlock()
	return r.lastLogMessage
}

func initiateFirehoseConnection() (*consumer.Consumer, <-chan *events.Envelope) {
	localIP, _ := localip.LocalIP()
	firehoseConnection := consumer.New("ws://"+localIP+":49629", &tls.Config{InsecureSkipVerify: true}, nil)
	msgChan, _ := firehoseConnection.Firehose("uniqueId", "")
	return firehoseConnection, msgChan
}

func testMetric(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_ValueMetric && msg.ValueMetric.GetName() == "fake-metric-name"
}

func metronSentMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "DopplerForwarder.sentMessages" && msg.GetOrigin() == "MetronAgent"
}

func dopplerReceivedMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && (msg.CounterEvent.GetName() == "dropsondeListener.receivedMessageCount" || msg.CounterEvent.GetName() == "tlsListener.receivedMessageCount") && msg.GetOrigin() == "DopplerServer"
}

func dopplerSentMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && strings.HasPrefix(msg.CounterEvent.GetName(), "sentMessagesFirehose") && msg.GetOrigin() == "DopplerServer"
}

func isLogMessage(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_LogMessage
}
