package proxy_test

import (
	"time"
	"trafficcontroller/internal/proxy"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewDopplerEndpoint", func() {
	Context("when endpoint is 'recentlogs'", func() {
		It("uses an HTTP handler", func() {
			dopplerEndpoint := proxy.NewDopplerEndpoint("recentlogs", "abc123", true)
			knownHttpHandler := proxy.NewHttpHandler(nil)
			Expect(dopplerEndpoint.HProvider(nil)).To(BeAssignableToTypeOf(knownHttpHandler))
		})

		It("sets a timeout of five seconds", func() {
			dopplerEndpoint := proxy.NewDopplerEndpoint("recentlogs", "abc123", true)
			Expect(dopplerEndpoint.Timeout).To(Equal(5 * time.Second))
		})
	})

	Context("when endpoint is 'containermetrics'", func() {
		It("uses an HTTP Container metrics handler", func() {
			dopplerEndpoint := proxy.NewDopplerEndpoint("containermetrics", "abc123", true)
			handler := proxy.NewHttpHandler(nil)
			closedChan := make(chan []byte)
			close(closedChan)
			Expect(dopplerEndpoint.HProvider(closedChan)).To(BeAssignableToTypeOf(handler))
		})

		It("sets a timeout of five seconds", func() {
			dopplerEndpoint := proxy.NewDopplerEndpoint("containermetrics", "abc123", true)
			Expect(dopplerEndpoint.Timeout).To(Equal(5 * time.Second))
		})
	})

	Context("when endpoint is not 'recentlogs'", func() {
		It("defaults to never timing out", func() {
			dopplerEndpoint := proxy.NewDopplerEndpoint("firehose", "firehose", true)
			Expect(dopplerEndpoint.Timeout).To(Equal(time.Duration(0)))
		})
	})
})

var _ = Describe("GetPath", func() {
	It("returns correct path for firehose", func() {
		dopplerEndpoint := proxy.NewDopplerEndpoint("firehose", "subscription-123", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/firehose/subscription-123"))
	})

	It("returns correct path for recentlogs", func() {
		dopplerEndpoint := proxy.NewDopplerEndpoint("recentlogs", "abc123", true)
		Expect(dopplerEndpoint.GetPath()).To(Equal("/apps/abc123/recentlogs"))
	})
})

var _ = Describe("ContainerMetricsHandler", func() {
	It("removes duplicate app container metrics", func() {
		messagesChan := make(chan []byte, 2)

		env1, _ := emitter.Wrap(factories.NewContainerMetric("1", 1, 123, 123, 123), "origin")
		env1.Timestamp = proto.Int64(10000)

		env2, _ := emitter.Wrap(factories.NewContainerMetric("1", 1, 123, 123, 123), "origin")
		env2.Timestamp = proto.Int64(20000)

		bytes1, _ := proto.Marshal(env1)
		bytes2, _ := proto.Marshal(env2)

		messagesChan <- bytes2
		messagesChan <- bytes1
		close(messagesChan)

		outputChan := proxy.DeDupe(messagesChan)

		Expect(outputChan).To(HaveLen(1))
		Expect(outputChan).To(Receive(Equal(bytes2)))
		Expect(outputChan).To(BeClosed())
	})

})
