package conversion_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tov1", func() {
	It("doesn't modify the input data", func() {
		tags := make(map[string]string)
		dTags := make(map[string]*loggregator_v2.Value)

		tags["foo"] = "bar"
		dTags["foo"] = &loggregator_v2.Value{Data: &loggregator_v2.Value_Text{Text: "baz"}}

		v2e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{
					Payload: []byte("hello"),
				},
			},
			Tags:           tags,
			DeprecatedTags: dTags,
		}

		conversion.ToV1(v2e)
		Expect(v2e.Tags["foo"]).To(Equal("bar"))
	})

	It("sets the application id to nil when it's not parsable into a UUID", func() {
		v2e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Timer{
				Timer: &loggregator_v2.Timer{},
			},
			SourceId: "some-id",
		}
		v1e := conversion.ToV1(v2e)
		Expect(v1e[0].HttpStartStop.ApplicationId).To(BeNil())
	})
})
