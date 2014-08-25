package envelopewrapper_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation/testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EnvelopeWrapper", func() {
	var (
		inputChan   chan *events.Envelope
		outputChan  chan *envelopewrapper.WrappedEnvelope
		runComplete chan struct{}
		marshaller  envelopewrapper.EnvelopeWrapper
	)

	BeforeEach(func() {
		inputChan = make(chan *events.Envelope, 10)
		outputChan = make(chan *envelopewrapper.WrappedEnvelope, 10)
		runComplete = make(chan struct{})
		marshaller = envelopewrapper.NewEnvelopeWrapper(loggertesthelper.Logger())

		go func() {
			marshaller.Run(inputChan, outputChan)
			close(runComplete)
		}()
	})

	AfterEach(func() {
		close(inputChan)
		Eventually(runComplete).Should(BeClosed())
	})

	It("wraps envelopes", func() {
		envelope := &events.Envelope{
			Origin:    proto.String("fake-origin-3"),
			EventType: events.Envelope_Heartbeat.Enum(),
			Heartbeat: factories.NewHeartbeat(1, 2, 3),
		}
		envelopeBytes, _ := proto.Marshal(envelope)

		expectedOutput := &envelopewrapper.WrappedEnvelope{Envelope: envelope, EnvelopeBytes: envelopeBytes}

		inputChan <- envelope
		output := <-outputChan
		Expect(output).To(Equal(expectedOutput))
	})

	Context("metrics", func() {
		It("emits the correct metrics context", func() {
			Expect(marshaller.Emit().Name).To(Equal("envelopeWrapper"))
		})

		It("emits a marshal error counter", func() {
			envelope := &events.Envelope{}

			inputChan <- envelope
			testhelpers.EventuallyExpectMetric(marshaller, "marshalErrors", 1)
		})
	})
})

var _ = Describe("WrappedEnvelope", func() {
	Describe("EnvelopeLength", func() {
		It("returns the length of EnvelopeBytes", func() {
			we := envelopewrapper.WrappedEnvelope{
				EnvelopeBytes: []byte{0, 1, 2},
			}
			Expect(we.EnvelopeLength()).To(Equal(3))
		})
	})
})

var _ = Describe("WrapEvent", func() {
	It("works in the happy case", func() {
		event := factories.NewHeartbeat(1, 2, 3)
		we, err := envelopewrapper.WrapEvent(event, "origin")
		Expect(err).NotTo(HaveOccurred())

		Expect(we.Envelope.GetEventType()).To(Equal(events.Envelope_Heartbeat))

		newEnvelope := &events.Envelope{}
		proto.Unmarshal(we.EnvelopeBytes, newEnvelope)
		Expect(newEnvelope).To(Equal(we.Envelope))
	})

	It("errors without an origin", func() {
		event := factories.NewHeartbeat(1, 2, 3)
		we, err := envelopewrapper.WrapEvent(event, "")
		Expect(err).To(Equal(emitter.ErrorMissingOrigin))
		Expect(we).To(BeNil())
	})

	It("errors when wrapping an incomplete event", func() {
		event := &events.Heartbeat{}
		we, err := envelopewrapper.WrapEvent(event, "origin")
		Expect(err).To(BeAssignableToTypeOf(&proto.RequiredNotSetError{}))
		Expect(we).To(BeNil())
	})
})
