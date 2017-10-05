package doppler_test

import (
	"fmt"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/testservers"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("V2 Egress", func() {
	Context("emitting into v1 Ingress", func() {
		It("receives envelope batches", func() {
			etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
			defer etcdCleanup()
			dopplerCleanup, dopplerPorts := testservers.StartDoppler(
				testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := dopplerIngressV1Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
			defer ingressCleanup()
			egressCleanup, egressClient := dopplerEgressV2Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			receiverClient, err := egressClient.BatchedReceiver(ctx, &v2.EgressBatchRequest{})
			Expect(err).ToNot(HaveOccurred())
			defer receiverClient.CloseSend()

			done := make(chan struct{})
			defer close(done)
			go func() {
				for {
					_ = sendAppLog("some-test-app-id", "message", ingressClient)

					select {
					case <-time.After(500 * time.Millisecond):
					case <-done:
						return
					}
				}
			}()

			Eventually(func() []*v2.Envelope {
				resp, err := receiverClient.Recv()
				Expect(err).ToNot(HaveOccurred())
				if err != nil {
					return nil
				}

				return resp.GetBatch()
			}).ShouldNot(BeEmpty())
		})
	})

	Context("emitting into v2 Ingress", func() {
		It("receives envelope batches", func() {
			etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
			defer etcdCleanup()
			dopplerCleanup, dopplerPorts := testservers.StartDoppler(
				testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := dopplerIngressV2Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
			defer ingressCleanup()
			egressCleanup, egressClient := dopplerEgressV2Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			receiverClient, err := egressClient.BatchedReceiver(ctx, &v2.EgressBatchRequest{})
			Expect(err).ToNot(HaveOccurred())
			defer receiverClient.CloseSend()

			done := make(chan struct{})
			defer close(done)
			go func() {
				for {
					_ = sendV2AppLog("some-test-app-id", "message", ingressClient)

					select {
					case <-time.After(500 * time.Millisecond):
					case <-done:
						return
					}
				}
			}()

			Eventually(func() []*v2.Envelope {
				resp, err := receiverClient.Recv()
				Expect(err).ToNot(HaveOccurred())
				if err != nil {
					return nil
				}

				return resp.GetBatch()
			}).ShouldNot(BeEmpty())
		})
	})
})

func sendV2AppLog(appID, msg string, ingressClient v2.DopplerIngress_SenderClient) error {
	return ingressClient.Send(&v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		Message: &v2.Envelope_Log{
			Log: &v2.Log{
				Payload: []byte(msg),
			},
		},
	})
}
