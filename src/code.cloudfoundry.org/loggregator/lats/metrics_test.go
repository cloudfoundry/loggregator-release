package lats_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sending metrics through loggregator", func() {
	Describe("Firehose", func() {
		var (
			msgChan   <-chan *events.Envelope
			errorChan <-chan error
		)
		BeforeEach(func() {
			msgChan, errorChan = ConnectToFirehose()
		})

		AfterEach(func() {
			Expect(errorChan).To(BeEmpty())
		})

		It("receives a counter event with correct total", func() {
			envelope := createCounterEvent()
			EmitToMetronV1(envelope)

			receivedEnvelope := FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent()).To(Equal(envelope.GetCounterEvent()))
			EmitToMetronV1(envelope)

			receivedEnvelope = FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent().GetTotal()).To(Equal(uint64(10)))
		})

		It("receives a value metric", func() {
			envelope := createValueMetric()
			EmitToMetronV1(envelope)

			receivedEnvelope := FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetValueMetric()).To(Equal(envelope.GetValueMetric()))
		})

		It("receives a container metric", func() {
			envelope := createContainerMetric("test-id")
			EmitToMetronV1(envelope)

			receivedEnvelope := FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
		})
	})

	Describe("Stream", func() {
		It("receives a container metric", func() {
			msgChan, errorChan := ConnectToStream("test-id")
			envelope := createContainerMetric("test-id")
			EmitToMetronV1(createContainerMetric("alternate-id"))
			EmitToMetronV1(envelope)

			receivedEnvelope, err := FindMatchingEnvelopeByID("test-id", msgChan)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
			Expect(errorChan).To(BeEmpty())
		})
	})
})
