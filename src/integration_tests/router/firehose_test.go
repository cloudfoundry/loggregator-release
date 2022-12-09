package router_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/integration_tests/fakes"
	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"code.cloudfoundry.org/loggregator-release/src/testservers"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firehose test", func() {
	Context("gRPC ingress/egress", func() {
		It("receives log messages", func() {
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
			subscribeClient, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV1(ingressClient, subscribeClient)

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

			Eventually(func() events.Envelope_EventType {
				resp, err := subscribeClient.Recv()
				if err != nil {
					return 0
				}
				return decodeProtoBufEnvelope(resp.GetPayload()).GetEventType()
			}).Should(Equal(events.Envelope_LogMessage))
		})

		It("two separate firehose subscriptions receive the same message", func() {
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
			subscribeClient0, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id-0",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient0.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV1(ingressClient, subscribeClient0)
			subscribeClient1, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id-1",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient1.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV1(ingressClient, subscribeClient1)

			err = sendAppLog("some-test-app-id", "message", ingressClient)
			Expect(err).ToNot(HaveOccurred())

			f := func(subscribeClient plumbing.Doppler_SubscribeClient) func() string {
				return func() string {
					resp, err := subscribeClient.Recv()
					if err != nil {
						return ""
					}

					return string(decodeProtoBufEnvelope(resp.GetPayload()).
						GetLogMessage().
						GetMessage())
				}
			}
			Eventually(f(subscribeClient0)).Should(Equal("message"))
			Eventually(f(subscribeClient1)).Should(Equal("message"))
		})

		It("firehose subscriptions split message load", func() {
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
			subscribeClient0, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient0.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV1(ingressClient, subscribeClient0)
			subscribeClient1, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient1.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV1(ingressClient, subscribeClient1)

			for i := 0; i < 100; i++ {
				err = sendAppLog("some-test-app-id", "message", ingressClient)
				Expect(err).ToNot(HaveOccurred())
			}
			sub0Messages, sub1Messages := make(chan struct{}, 100), make(chan struct{}, 100)
			recvMessages(sub0Messages, subscribeClient0)
			recvMessages(sub1Messages, subscribeClient1)

			Eventually(func() int {
				return len(sub0Messages) + len(sub1Messages)
			}).Should(Equal(100))

			Expect(len(sub0Messages) - len(sub1Messages)).To(BeNumerically("~", 0, 30))
		})

		It("does not receive duplicate logs for missing app ID", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV2Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			subscribeClient, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV2(ingressClient, subscribeClient)

			err = ingressClient.Send(&loggregator_v2.Envelope{
				Timestamp: time.Now().UnixNano(),
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("hello world"),
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			envs := make(chan *events.Envelope, 100)
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				for {
					defer GinkgoRecover()
					msg, err := subscribeClient.Recv()
					if err != nil {
						return
					}

					e := unmarshalMessage(msg.Payload)
					// filter out primer messages
					if string(e.GetLogMessage().GetMessage()) != "hello world" {
						continue
					}

					select {
					case <-stop:
						return
					case envs <- e:
					}
				}
			}()

			Eventually(envs, 5).Should(HaveLen(1))
			Consistently(envs).Should(HaveLen(1))
		})

		It("does not receive duplicate logs for missing app ID with a filter", func() {
			dopplerCleanup, dopplerPorts := testservers.StartRouter(
				testservers.BuildRouterConfig(0, 0),
			)
			defer dopplerCleanup()
			ingressCleanup, ingressClient := fakes.DopplerIngressV2Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer ingressCleanup()
			egressCleanup, egressClient := fakes.DopplerEgressV1Client(fmt.Sprintf("127.0.0.1:%d", dopplerPorts.GRPC))
			defer egressCleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			subscribeClient, err := egressClient.Subscribe(
				ctx,
				&plumbing.SubscriptionRequest{
					ShardID: "shard-id",
					Filter: &plumbing.Filter{
						Message: &plumbing.Filter_Log{
							Log: &plumbing.LogFilter{},
						},
					},
				},
			)
			Expect(err).ToNot(HaveOccurred())
			defer func() {
				err := subscribeClient.CloseSend()
				Expect(err).ToNot(HaveOccurred())
			}()
			primePumpV2(ingressClient, subscribeClient)

			err = ingressClient.Send(&loggregator_v2.Envelope{
				Timestamp: time.Now().UnixNano(),
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("hello world"),
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			envs := make(chan *events.Envelope, 100)
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				for {
					defer GinkgoRecover()
					msg, err := subscribeClient.Recv()
					if err != nil {
						return
					}

					e := unmarshalMessage(msg.Payload)
					// filter out primer messages
					if string(e.GetLogMessage().GetMessage()) != "hello world" {
						continue
					}

					select {
					case <-stop:
						return
					case envs <- e:
						// Do Nothing
					}
				}
			}()

			Eventually(envs, 5).Should(HaveLen(1))
			Consistently(envs).Should(HaveLen(1))
		})
	})
})

func recvMessages(recv chan struct{}, client plumbing.Doppler_SubscribeClient) {
	go func() {
		for {
			resp, err := client.Recv()
			if err != nil {
				return
			}
			// filter out primer messages
			if bytes.Contains(resp.Payload, []byte("primer")) {
				continue
			}
			recv <- struct{}{}
		}
	}()
}
