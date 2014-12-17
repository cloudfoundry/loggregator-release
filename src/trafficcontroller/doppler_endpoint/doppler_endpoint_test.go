package doppler_endpoint_test

import (
	"trafficcontroller/doppler_endpoint"

	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewDopplerEndpoint", func() {
	Context("when endpoint is 'recentlogs'", func() {
		It("uses an HTTP handler", func() {
			dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("recentlogs", "abc123", true)
			knownHttpHandler := handlers.NewHttpHandler(nil, nil)
			Expect(dopplerEndpoint.HProvider(nil, nil)).To(BeAssignableToTypeOf(knownHttpHandler))
		})

		It("sets a timeout of five seconds", func() {
			dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("recentlogs", "abc123", true)
			Expect(dopplerEndpoint.Timeout).To(Equal(5 * time.Second))
		})
	})

	Context("when endpoint is not 'recentlogs'", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("firehose", "firehose", true)
		Expect(dopplerEndpoint.Timeout).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("GetPath", func() {
	It("returns correct path for firehose", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("firehose", "subscription-123", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/firehose/subscription-123"))
	})

	It("returns correct path for recentlogs", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("recentlogs", "abc123", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/apps/abc123/recentlogs"))
	})
})
