package lats_test

import (
	"code.cloudfoundry.org/loggregator/lats/helpers"

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
			msgChan, errorChan = helpers.ConnectToFirehose()
		})

		AfterEach(func() {
			Expect(errorChan).To(BeEmpty())
		})

		It("receives a counter event with correct total", func() {
			envelope := createCounterEvent()
			helpers.EmitToMetronV1(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent()).To(Equal(envelope.GetCounterEvent()))
			helpers.EmitToMetronV1(envelope)

			receivedEnvelope = helpers.FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent().GetTotal()).To(Equal(uint64(10)))
		})

		It("receives a value metric", func() {
			envelope := createValueMetric()
			helpers.EmitToMetronV1(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetValueMetric()).To(Equal(envelope.GetValueMetric()))
		})

		It("receives a container metric", func() {
			envelope := createContainerMetric("test-id")
			helpers.EmitToMetronV1(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan, envelope)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
		})
	})

	Describe("Stream", func() {
		It("receives a container metric", func() {
			msgChan, errorChan := helpers.ConnectToStream("test-id")
			envelope := createContainerMetric("test-id")
			helpers.EmitToMetronV1(createContainerMetric("alternate-id"))
			helpers.EmitToMetronV1(envelope)

			receivedEnvelope, err := helpers.FindMatchingEnvelopeByID("test-id", msgChan)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
			Expect(errorChan).To(BeEmpty())
		})
	})
})
