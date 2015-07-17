package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"github.com/gogo/protobuf/proto"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/noaa"
	"crypto/tls"
	"time"
)

const (
	numMessagesSent = 1000
)

var _ = Describe("Simple test", func() {

	Measure("dropsonde metrics being passed from metron to the firehose nozzle", func(b Benchmarker) {
		metronInput := initiateMetronConnection()
		msgChan := initiateFirehoseConnection()

		var nonTestMetricCount float64
		var testMetricCount float64
		var metronMessageCount float64
		var dopplerMessageCount float64

		b.Time("runtime", func() {
			sendTestMetricsToMetron(metronInput)

			timeoutSeconds := 15 * time.Second
			timeoutChan := time.After(timeoutSeconds)

			timedOut := false
			metronMessageCountMetricReceived := false
			dopplerMessageCountMetricReceived := false

			for !(metronMessageCountMetricReceived && dopplerMessageCountMetricReceived) && !timedOut {
				select {
				case msg := <- msgChan:
					if isTestMetric(msg) {
						testMetricCount += 1
					} else {
						nonTestMetricCount += 1
					}

					if isMetronMessageCount(msg) {
						metronMessageCountMetricReceived = true
						metronMessageCount = float64(msg.CounterEvent.GetTotal())
					}

					if isDopplerMessageCount(msg) {
						dopplerMessageCountMetricReceived = true
						dopplerMessageCount = float64(msg.CounterEvent.GetTotal())
					}
				case <- timeoutChan:
					timedOut = true
				}
			}

			Expect(timedOut).To(BeFalse(), "Timed out before receiving necessary metrics from metron and doppler")
		})

		var totalMessagesReceivedByFirehose float64
		totalMessagesReceivedByFirehose = testMetricCount + nonTestMetricCount

		percentMessageLossBetweenMetronAndDoppler := computePercentLost(metronMessageCount, dopplerMessageCount)
		percentMessageLossBetweenDopplerAndFirehose := computePercentLost(dopplerMessageCount, totalMessagesReceivedByFirehose)
		percentMessageLossOverEntirePipeline := computePercentLost(metronMessageCount, totalMessagesReceivedByFirehose)

		b.RecordValue("Messages received by metron", metronMessageCount)
		b.RecordValue("Messages received by doppler", dopplerMessageCount)
		b.RecordValue("Test messages received by firehose nozzle", testMetricCount)
		b.RecordValue("Non-test messages received by firehose nozzle", nonTestMetricCount)
		b.RecordValue("Total messages received by firehose nozzle", totalMessagesReceivedByFirehose)

		b.RecordValue("Message Loss percentage between Metron and Doppler ", percentMessageLossBetweenMetronAndDoppler)
		b.RecordValue("Message Loss percentage between Doppler and Firehose nozzle", percentMessageLossBetweenDopplerAndFirehose)
		b.RecordValue("Total message loss percentage (received by metron but not received by the firehose)", percentMessageLossOverEntirePipeline)

		Expect(percentMessageLossBetweenMetronAndDoppler).To(BeNumerically("<", 5.0))
		Expect(percentMessageLossBetweenDopplerAndFirehose).To(BeNumerically("<", 5.0))
		Expect(percentMessageLossOverEntirePipeline).To(BeNumerically("<", 5.0))
		Expect(float64(metronMessageCount)).To(BeNumerically(">", numMessagesSent))
	}, 5)
})

func computePercentLost(totalMessages, receivedMessages float64) float64 {
	return ((totalMessages - receivedMessages) / totalMessages) * 100
}

func initiateFirehoseConnection() chan *events.Envelope {
	firehoseConnection := noaa.NewConsumer("ws://" + LocalIPAddress + ":49629", &tls.Config{InsecureSkipVerify: true}, nil)
	msgChan := make(chan *events.Envelope, 2 * numMessagesSent)
	errorChan := make(chan error)
	go firehoseConnection.Firehose("uniqueId", "", msgChan, errorChan)
	return msgChan
}

func initiateMetronConnection() net.Conn {
	metronInput, err := net.Dial("udp", "localhost:49625")
	Expect(err).ToNot(HaveOccurred())
	return metronInput
}

func sendTestMetricsToMetron(metronInput net.Conn) {
	for i := 0; i < numMessagesSent; i++ {
		message := basicValueMessage(i)

		bytesOut, err := metronInput.Write(message)
		Expect(err).ToNot(HaveOccurred())
		Expect(bytesOut).To(Equal(len(message)))
		time.Sleep(1 * time.Millisecond)
	}
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

func basicValueMessage(i int) []byte {
	message, _ := proto.Marshal(basicValueMessageEnvelope(i))
	return message
}

func basicValueMessageEnvelope(i int) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(float64(i)),
			Unit:  proto.String("fake-unit"),
		},
	}
}
