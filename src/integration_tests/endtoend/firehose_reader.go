package endtoend

import (
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"crypto/tls"
	"github.com/cloudfoundry/noaa"
	"github.com/pivotal-golang/localip"
)

type FirehoseReader struct{
	numMessagesSent int
	benchmarker Benchmarker
	msgChan chan *events.Envelope

	metronMessageCountMetricReceived bool
	dopplerMessageCountMetricReceived bool

	testMetricCount float64
	nonTestMetricCount float64
	metronMessageCount float64
	dopplerMessageCount float64
}


func NewFirehoseReader(b Benchmarker, numMessagesSent int) *FirehoseReader {
	return &FirehoseReader{
		benchmarker: b,
		numMessagesSent: numMessagesSent,
		msgChan: initiateFirehoseConnection(numMessagesSent),
		testMetricCount: 0,
		nonTestMetricCount: 0,
		metronMessageCount: 0,
		dopplerMessageCount: 0,
	}
}

func(r *FirehoseReader) Read() {
	for msg := range r.msgChan {
		if isTestMetric(msg) {
			r.testMetricCount += 1
		} else {
			r.nonTestMetricCount += 1
		}

		if isMetronMessageCount(msg) {
			r.metronMessageCountMetricReceived = true
			r.metronMessageCount = float64(msg.CounterEvent.GetTotal())
		}

		if isDopplerMessageCount(msg) {
			r.dopplerMessageCountMetricReceived = true
			r.dopplerMessageCount = float64(msg.CounterEvent.GetTotal())
		}
	}

}

func (r *FirehoseReader) Report(){
	var totalMessagesReceivedByFirehose float64
	totalMessagesReceivedByFirehose = r.testMetricCount + r.nonTestMetricCount

	percentMessageLossBetweenMetronAndDoppler := computePercentLost(r.metronMessageCount, r.dopplerMessageCount)
	percentMessageLossBetweenDopplerAndFirehose := computePercentLost(r.dopplerMessageCount, totalMessagesReceivedByFirehose)
	percentMessageLossOverEntirePipeline := computePercentLost(r.metronMessageCount, totalMessagesReceivedByFirehose)

	r.benchmarker.RecordValue("Messages received by metron", r.metronMessageCount)
	r.benchmarker.RecordValue("Messages received by doppler", r.dopplerMessageCount)
	r.benchmarker.RecordValue("Test messages received by firehose nozzle", r.testMetricCount)
	r.benchmarker.RecordValue("Non-test messages received by firehose nozzle", r.nonTestMetricCount)
	r.benchmarker.RecordValue("Total messages received by firehose nozzle", totalMessagesReceivedByFirehose)

	r.benchmarker.RecordValue("Message Loss percentage between Metron and Doppler ", percentMessageLossBetweenMetronAndDoppler)
	r.benchmarker.RecordValue("Message Loss percentage between Doppler and Firehose nozzle", percentMessageLossBetweenDopplerAndFirehose)
	r.benchmarker.RecordValue("Total message loss percentage (received by metron but not received by the firehose)", percentMessageLossOverEntirePipeline)

	Expect(percentMessageLossBetweenMetronAndDoppler).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossBetweenDopplerAndFirehose).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossOverEntirePipeline).To(BeNumerically("<", 5.0))
	Expect(float64(r.metronMessageCount)).To(BeNumerically(">", r.numMessagesSent))
}

func initiateFirehoseConnection(numMessagesSent int) chan *events.Envelope {
	localIP, _ := localip.LocalIP()
	firehoseConnection := noaa.NewConsumer("ws://" + localIP + ":49629", &tls.Config{InsecureSkipVerify: true}, nil)
	msgChan := make(chan *events.Envelope, 2 * numMessagesSent)
	errorChan := make(chan error)
	go firehoseConnection.Firehose("uniqueId", "", msgChan, errorChan)
	return msgChan
}

func computePercentLost(totalMessages, receivedMessages float64) float64 {
	return ((totalMessages - receivedMessages) / totalMessages) * 100
}

func isTestMetric(msg *events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_ValueMetric && msg.ValueMetric.GetName() == "fake-metric-name"
}

func isMetronMessageCount(msg * events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "dropsondeAgentListener.receivedMessageCount"
}

func isDopplerMessageCount(msg * events.Envelope) bool {
	return msg.GetEventType() == events.Envelope_CounterEvent && msg.CounterEvent.GetName() == "dropsondeListener.receivedMessageCount"
}