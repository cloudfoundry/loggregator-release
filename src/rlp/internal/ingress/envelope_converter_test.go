package ingress_test

import (
	"rlp/internal/ingress"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Converter", func() {
	It("converts bytes to v2 envelopes", func() {
		c := ingress.NewConverter()

		envelopeBytes, _ := (&events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_LogMessage.Enum(),
		}).Marshal()
		v2e, err := c.Convert(envelopeBytes)

		Expect(err).ToNot(HaveOccurred())
		Expect(v2e.GetDeprecatedTags()["origin"].GetText()).To(Equal("some-origin"))
	})

	It("returns an error when unmarshalling fails", func() {
		c := ingress.NewConverter()

		_, err := c.Convert([]byte("bad-envelope"))

		Expect(err).To(HaveOccurred())
	})
})
