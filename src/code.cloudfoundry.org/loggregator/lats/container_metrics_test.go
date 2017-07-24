package lats_test

import (
	"code.cloudfoundry.org/loggregator/lats/helpers"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("Container Metrics Endpoint", func() {
	It("can receive container metrics", func() {
		envelope := createContainerMetric("test-id")
		helpers.EmitToMetronV1(envelope)

		f := func() []*events.ContainerMetric {
			return helpers.RequestContainerMetrics("test-id")
		}
		Eventually(f).Should(ContainElement(envelope.ContainerMetric))
	})

	Describe("emit v2 and consume via reverse log proxy", func() {
		It("can receive container metrics", func() {
			envelope := createContainerMetric("test-id")
			v2Env := conversion.ToV2(envelope, false)
			helpers.EmitToMetronV2(v2Env)

			f := func() []*v2.Envelope {
				return helpers.ReadContainerFromRLP("test-id", false)
			}
			Eventually(f).Should(ContainElement(v2Env))
		})

		It("can receive container metrics with preferred tags", func() {
			envelope := createContainerMetric("test-id")
			v2Env := conversion.ToV2(envelope, true)
			helpers.EmitToMetronV2(v2Env)

			f := func() []*v2.Envelope {
				return helpers.ReadContainerFromRLP("test-id", true)
			}
			Eventually(f).Should(ContainElement(v2Env))
		})
	})
})
