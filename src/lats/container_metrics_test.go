package lats_test

import (
	"lats/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("Container Metrics Endpoint", func() {
	It("can receive container metrics", func() {
		envelope := createContainerMetric("test-id")
		helpers.EmitToMetron(envelope)

		f := func() []*events.ContainerMetric {
			resp, _ := helpers.RequestContainerMetrics("test-id")
			if len(resp) == 0 {
				return nil
			}
			return resp
		}
		Eventually(f).Should(ContainElement(envelope.ContainerMetric))
	})
})
