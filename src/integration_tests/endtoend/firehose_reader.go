package endtoend

import (
	"crypto/tls"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/pivotal-golang/localip"
	"strings"
)

type FirehoseReader struct {
	consumer *noaa.Consumer
	msgChan  chan *events.Envelope
	stopChan chan struct{}

	TestMetricCount             float64
	NonTestMetricCount          float64
	MetronSentMessageCount      float64
	DopplerReceivedMessageCount float64
	DopplerSentMessageCount     float64
}

func NewFirehoseReader() *FirehoseReader {
	consumer, msgChan := initiateFirehoseConnection()
	return &FirehoseReader{
		consumer:                    consumer,
		msgChan:                     msgChan,
		stopChan:                    make(chan struct{}),
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
	}
}

func (r *FirehoseReader) Close() {
	close(r.stopChan)
	r.consumer.Close()
}

func initiateFirehoseConnection() (*noaa.Consumer, chan *events.Envelope) {
	localIP, _ := localip.LocalIP()
	firehoseConnection := noaa.NewConsumer("ws://"+localIP+":49629", &tls.Config{InsecureSkipVerify: true}, nil)
	msgChan := make(chan *events.Envelope, 2000)
	errorChan := make(chan error)
	go firehoseConnection.Firehose("uniqueId", "", msgChan, errorChan)
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
