package endtoend_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator-release/src/integration_tests/endtoend"
	"code.cloudfoundry.org/loggregator-release/src/integration_tests/fakes"
	"code.cloudfoundry.org/loggregator-release/src/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End to end tests", func() {
	It("sends messages from agent through doppler and traffic controller", func() {
		dopplerCleanup, dopplerPorts := testservers.StartRouter(
			testservers.BuildRouterConfig(0, 0),
		)
		defer dopplerCleanup()

		ingressCleanup, ingressClient := fakes.DopplerIngressV2Client(
			fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
		)
		defer ingressCleanup()

		trafficcontrollerCleanup, tcPorts := testservers.StartTrafficController(
			testservers.BuildTrafficControllerConf(
				fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
				0,
			),
		)
		defer trafficcontrollerCleanup()

		firehoseReader := endtoend.NewFirehoseReader(tcPorts.WS)

		go func() {
			for range time.Tick(time.Millisecond) {
				_ = ingressClient.Send(endtoend.BasicLogMessageEnvelopeV2("custom-app-id"))
			}
		}()

		go func() {
			for {
				firehoseReader.Read()
			}
		}()

		Eventually(firehoseReader.LogMessageAppIDs, 5).Should(Receive(Equal("custom-app-id")))
	}, 10)
})
