package metron_test

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metron", func() {
	It("writes downstream via gRPC", func() {
		etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
		defer etcdCleanup()
		dopplerCleanup, dopplerPorts := testservers.StartDoppler(
			testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
		)
		defer dopplerCleanup()
		metronCleanup, metronPorts := testservers.StartMetron(
			testservers.BuildMetronConfig("localhost", dopplerPorts.GRPC),
		)
		defer metronCleanup()
		egressCleanup, egressClient := dopplerEgressClient(fmt.Sprintf("localhost:%d", dopplerPorts.GRPC))
		defer egressCleanup()

		err := dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronPorts.UDP), "test-origin")
		Expect(err).NotTo(HaveOccurred())
		subscriptionClient, err := egressClient.Subscribe(
			context.Background(),
			&plumbing.SubscriptionRequest{
				ShardID: "shard-id",
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("sending a message into metron")
		err = logs.SendAppLog("test-app-id", "An event happened!", "test-app-id", "0")
		Expect(err).NotTo(HaveOccurred())

		By("reading a message from doppler")

		Eventually(func() ([]byte, error) {
			resp, err := subscriptionClient.Recv()
			if err != nil {
				return nil, err
			}
			return resp.GetPayload(), nil
		}).Should(ContainSubstring("An event happened!"))
	})
})

func dopplerEgressClient(addr string) (func(), plumbing.DopplerClient) {
	creds, err := plumbing.NewClientCredentials(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
		"doppler",
	)
	Expect(err).ToNot(HaveOccurred())

	out, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	Expect(err).ToNot(HaveOccurred())
	return func() {
		_ = out.Close()
	}, plumbing.NewDopplerClient(out)
}
