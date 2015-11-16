package truncatingbuffer_test

import (
	. "truncatingbuffer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("BufferContext", func() {
	Context("DefaultContext", func(){
		var defaultContext *DefaultContext

		BeforeEach(func(){
			defaultContext = NewDefaultContext("origin", "testIdentifier")
		})

		It("Should return a valid properties", func(){
			Expect(defaultContext.DropsondeOrigin()).To(Equal("origin"))
			Expect(defaultContext.Identifier()).To(Equal("testIdentifier"))
			for _, event := range events.Envelope_EventType_value {
				Expect(defaultContext.EventAllowed(events.Envelope_EventType(event))).To(BeTrue())
			}
		})
	})

	Context("LogAllowedContext", func(){
		var logAllowedContext *LogAllowedContext

		BeforeEach(func(){
			logAllowedContext = NewLogAllowedContext("origin", "testIdentifier")
		})

		It("Should return a valid properties", func(){
			Expect(logAllowedContext.DropsondeOrigin()).To(Equal("origin"))
			Expect(logAllowedContext.Identifier()).To(Equal("testIdentifier"))
			for _, e := range events.Envelope_EventType_value {
				event := events.Envelope_EventType(e)
				allowed := logAllowedContext.EventAllowed(event)
				if event == events.Envelope_LogMessage {
					Expect(allowed).To(BeTrue())
				} else {
					Expect(allowed).To(BeFalse())

				}
			}
		})
	})
})
