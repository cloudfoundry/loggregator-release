package doppler_endpoint_test

import (
	"trafficcontroller/doppler_endpoint"

	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewDopplerEndpoint", func() {
	It("when endpoint is 'recentlogs', uses an HTTP handler", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("recentlogs", "abc123", true)
		knownHttpHandler := handlers.NewHttpHandler(nil, nil)
		Expect(dopplerEndpoint.HProvider(nil, nil)).To(BeAssignableToTypeOf(knownHttpHandler))
	})

	It("when endpoint is not 'recentlogs', uses an socket handler", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("firehose", "firehost", true)
		knownWebsocketHandler := handlers.NewWebsocketHandler(nil, 0, nil)
		Expect(dopplerEndpoint.HProvider(nil, nil)).To(BeAssignableToTypeOf(knownWebsocketHandler))
	})
})

var _ = Describe("GetPath", func() {
	It("returns correct path for firehose", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("firehose", "firehose", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/firehose"))
	})

	It("returns correct path for recentlogs", func() {
		dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint("recentlogs", "abc123", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/apps/abc123/recentlogs"))
	})
})
