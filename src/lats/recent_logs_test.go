package lats_test

import (
	"lats/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("Recent Logs Endpoint", func() {
	It("can receive recent logs for an app", func() {
		envelope := createLogMessage("test-id")
		helpers.EmitToMetron(envelope)

		f := func() []*events.LogMessage {
			resp, _ := helpers.RequestRecentLogs("test-id")
			if len(resp) == 0 {
				return nil
			}
			return resp
		}
		Eventually(f).Should(ContainElement(envelope.LogMessage))
	})
})
