package endtoend

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

type FirehoseReader struct {
	consumer *consumer.Consumer
	msgChan  <-chan *events.Envelope
	stopChan chan struct{}

	TestMetricCount             float64
	NonTestMetricCount          float64
	AgentSentMessageCount       float64
	DopplerReceivedMessageCount float64
	DopplerSentMessageCount     float64

	LogMessageAppIDs chan string
}

func NewFirehoseReader(tcPort int) *FirehoseReader {
	consumer, msgChan := initiateFirehoseConnection(tcPort)
	return &FirehoseReader{
		consumer:         consumer,
		msgChan:          msgChan,
		LogMessageAppIDs: make(chan string, 100),
		stopChan:         make(chan struct{}),
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

		if agentSentMessageCount(msg) {
			r.AgentSentMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if dopplerReceivedMessageCount(msg) {
			r.DopplerReceivedMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if dopplerSentMessageCount(msg) {
			r.DopplerSentMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if msg.GetEventType() == events.Envelope_LogMessage {
			select {
			case r.LogMessageAppIDs <- msg.GetLogMessage().GetAppId():
			default:
			}
		}
	}
}

func (r *FirehoseReader) Close() {
	close(r.stopChan)
	r.consumer.Close()
}

func initiateFirehoseConnection(tcPort int) (*consumer.Consumer, <-chan *events.Envelope) {
	localIP := "127.0.0.1"
	url := fmt.Sprintf("wss://%s:%d", localIP, tcPort)
	firehoseConnection := consumer.New(url, &tls.Config{InsecureSkipVerify: true}, nil)
	msgChan, _ := firehoseConnection.Firehose("uniqueId", "")
	return firehoseConnection, msgChan
}

func testMetric(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_ValueMetric && msg.ValueMetric.GetName() == "fake-metric-name"
}

func agentSentMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "DopplerForwarder.sentMessages" && msg.GetOrigin() == "MetronAgent"
}

func dopplerReceivedMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent &&
		(msg.CounterEvent.GetName() == "udpListener.receivedMessageCount" ||
			msg.CounterEvent.GetName() == "tcpListener.receivedMessageCount" ||
			msg.CounterEvent.GetName() == "tlsListener.receivedMessageCount") &&
		msg.GetOrigin() == "DopplerServer"
}

func dopplerSentMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && strings.HasPrefix(msg.CounterEvent.GetName(), "sentMessagesFirehose") && msg.GetOrigin() == "DopplerServer"
}
