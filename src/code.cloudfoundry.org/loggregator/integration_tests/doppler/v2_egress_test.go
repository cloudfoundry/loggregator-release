package doppler_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing/v2"
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
			receiverClient, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{})
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

			Eventually(func() []*loggregator_v2.Envelope {
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
		var (
			etcdCleanup    func()
			dopplerCleanup func()
			ingressCleanup func()
			egressCleanup  func()

			ingressClient loggregator_v2.DopplerIngress_SenderClient
			egressClient  loggregator_v2.EgressClient
		)

		BeforeEach(func() {
			var etcdClientURL string
			var dopplerPorts testservers.DopplerPorts

			etcdCleanup, etcdClientURL = testservers.StartTestEtcd()
			dopplerCleanup, dopplerPorts = testservers.StartDoppler(
				testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
			)
			ingressCleanup, ingressClient = dopplerIngressV2Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
			egressCleanup, egressClient = dopplerEgressV2Client(
				fmt.Sprintf("localhost:%d", dopplerPorts.GRPC),
			)
		})

		AfterEach(func() {
			egressCleanup()
			ingressCleanup()
			dopplerCleanup()
			etcdCleanup()
		})

		It("receives envelope batches", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			receiverClient, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{})
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

			Eventually(func() []*loggregator_v2.Envelope {
				resp, err := receiverClient.Recv()
				Expect(err).ToNot(HaveOccurred())
				if err != nil {
					return nil
				}

				return resp.GetBatch()
			}).ShouldNot(BeEmpty())
		})

		Describe("requests with selectors", func() {
			It("only returns envelopes of the requested type", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				receiverClient, err := egressClient.BatchedReceiver(
					ctx,
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					},
				)
				Expect(err).ToNot(HaveOccurred())
				defer receiverClient.CloseSend()

				done := make(chan struct{})
				defer close(done)
				go func() {
					for {
						_ = sendV2AppLog("some-test-app-id", "message", ingressClient)
						_ = sendV2Counter("some-test-app-id", ingressClient)

						select {
						case <-time.After(500 * time.Millisecond):
						case <-done:
							return
						}
					}
				}()

				var batch []*loggregator_v2.Envelope
				Eventually(func() []*loggregator_v2.Envelope {
					resp, err := receiverClient.Recv()
					Expect(err).ToNot(HaveOccurred())
					if err != nil {
						return nil
					}

					batch = resp.GetBatch()

					return resp.GetBatch()
				}).ShouldNot(BeEmpty())

				// Even though counters were emitted, we only get logs
				for _, e := range batch {
					Expect(e.GetLog()).ToNot(BeNil())
				}
			})
		})
	})
})

func sendV2AppLog(appID, msg string, ingressClient loggregator_v2.DopplerIngress_SenderClient) error {
	return ingressClient.Send(&loggregator_v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(msg),
			},
		},
	})
}

func sendV2Counter(appID string, ingressClient loggregator_v2.DopplerIngress_SenderClient) error {
	return ingressClient.Send(&loggregator_v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Counter{
			Counter: &loggregator_v2.Counter{
				Name:  "a-counter-name",
				Delta: 1234,
				Total: 12345,
			},
		},
	})
}
