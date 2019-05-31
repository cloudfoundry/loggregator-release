package router_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator/integration_tests/fakes"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
)

var _ = Describe("GRPC Streaming Logs", func() {
	Context("with a subscription established", func() {
		It("responds to a subscription request", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			req := &plumbing.SubscriptionRequest{
				ShardID: "foo",
				Filter: &plumbing.Filter{
					AppID: "some-test-app-id",
				},
			}
			subscribeClient, err := egressClient.Subscribe(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			defer subscribeClient.CloseSend()
			primePumpV1(ingressClient, subscribeClient)
			logMessage := buildLogMessage()

			err = ingressClient.Send(&plumbing.EnvelopeData{
				Payload: logMessage,
			})
			Expect(err).ToNot(HaveOccurred())

			f := func() []byte {
				msg, err := subscribeClient.Recv()
				Expect(err).ToNot(HaveOccurred())

				return msg.GetPayload()
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})
