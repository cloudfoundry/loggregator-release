package endtoend

import (
	"crypto/tls"

	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/pivotal-golang/localip"
)

type FirehoseReader struct {
	consumer *noaa.Consumer
	msgChan  chan *events.Envelope
	stopChan chan struct{}

	MetronMessageCountMetricReceived  bool
	DopplerMessageCountMetricReceived bool

	TestMetricCount     float64
	NonTestMetricCount  float64
	MetronMessageCount  float64
	DopplerMessageCount float64
}

func NewFirehoseReader() *FirehoseReader {
	consumer, msgChan := initiateFirehoseConnection()
	return &FirehoseReader{
		consumer:            consumer,
		msgChan:             msgChan,
		stopChan:            make(chan struct{}),
		TestMetricCount:     0,
		NonTestMetricCount:  0,
		MetronMessageCount:  0,
		DopplerMessageCount: 0,
	}
}

func (r *FirehoseReader) Read() {
	select {
	case <-r.stopChan:
		return
	case msg := <-r.msgChan:
		if isTestMetric(msg) {
			r.TestMetricCount += 1
		} else {
			r.NonTestMetricCount += 1
		}

		if isMetronMessageCount(msg) {
			r.MetronMessageCountMetricReceived = true
			r.MetronMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if isDopplerMessageCount(msg) {
			r.DopplerMessageCountMetricReceived = true
			r.DopplerMessageCount = float64(msg.CounterEvent.GetTotal())
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

func isTestMetric(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_ValueMetric && msg.ValueMetric.GetName() == "fake-metric-name"
}

func isMetronMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "dropsondeAgentListener.receivedMessageCount" && msg.GetOrigin() == "MetronAgent"
}

func isDopplerMessageCount(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "dropsondeListener.receivedMessageCount" && msg.GetOrigin() == "DopplerServer"
}
