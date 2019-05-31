package router_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/integration_tests/fakes"
	"code.cloudfoundry.org/loggregator/testservers"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("V2 Egress", func() {
	Context("emitting into v1 Ingress", func() {
		It("receives envelope batches", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV1Client(
				fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
			)
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV2Client(
				fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
			)
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			receiverClient, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			})
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
			dopplerCleanup func()
			ingressCleanup func()
			egressCleanup  func()

			ingressClient loggregator_v2.Ingress_SenderClient
			egressClient  loggregator_v2.EgressClient
		)

		BeforeEach(func() {
			var dopplerPorts testservers.RouterPorts

			dopplerCleanup, dopplerPorts = testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			ingressCleanup, ingressClient = fakes.DopplerIngressV2Client(
				fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
			)
			egressCleanup, egressClient = fakes.DopplerEgressV2Client(
				fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC),
			)
		})

		AfterEach(func() {
			egressCleanup()
			ingressCleanup()
			dopplerCleanup()
		})

		It("receives envelope batches", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			receiverClient, err := egressClient.BatchedReceiver(ctx, &loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			})
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

func sendV2AppLog(appID, msg string, ingressClient loggregator_v2.Ingress_SenderClient) error {
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

func sendV2Counter(appID string, ingressClient loggregator_v2.Ingress_SenderClient) error {
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
