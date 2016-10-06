package lats_test

import (
	"lats/helpers"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sending metrics through loggregator", func() {
	Describe("the firehose receives a", func() {
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

		It("counter event emitted to metron with correct total", func() {
			envelope := createCounterEvent()
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent()).To(Equal(envelope.GetCounterEvent()))
			helpers.EmitToMetron(envelope)

			receivedEnvelope = helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetCounterEvent().GetTotal()).To(Equal(uint64(10)))
		})

		It("value metric emitted to metron", func() {
			envelope := createValueMetric()
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetValueMetric()).To(Equal(envelope.GetValueMetric()))
		})

		It("container metric emitted to metron", func() {
			envelope := createContainerMetric("test-id")
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
		})
	})

	Describe("the stream receives a", func() {
		It("container metric emitted to metron", func() {
			msgChan, errorChan := helpers.ConnectToStream("test-id")
			envelope := createContainerMetric("test-id")
			helpers.EmitToMetron(createContainerMetric("alternate-id"))
			helpers.EmitToMetron(envelope)

			receivedEnvelope, err := helpers.FindMatchingEnvelopeById("test-id", msgChan)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedEnvelope).NotTo(BeNil())

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(envelope.GetContainerMetric()))
			Expect(errorChan).To(BeEmpty())
		})
	})
})
