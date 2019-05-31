package egress_test

import (
	"errors"
	"io"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/rlp/internal/egress"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	Describe("Receiver()", func() {
		It("errors when streams exceeds maxStreams", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(1)

			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
				egress.WithMaxStreams(0),
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			Expect(err).To(MatchError(
				status.Errorf(
					codes.ResourceExhausted,
					"unable to create stream, max egress streams reached: 0",
				),
			))
		})

		It("errors when the sender cannot send the envelope", func() {
			receiverServer := newSpyReceiverServer(errors.New("Oh No!"))
			receiver := newSpyReceiver(1)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			Expect(err).To(Equal(io.ErrUnexpectedEOF))
		})

		It("errors when there aren't any Message selectors", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(1)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{}, receiverServer)

			Expect(err).To(HaveOccurred())

			err = server.Receiver(&loggregator_v2.EgressRequest{
				Selectors: []*loggregator_v2.Selector{{}},
			}, receiverServer)

			Expect(err).To(HaveOccurred())
		})

		It("streams data when there are envelopes", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)
			Expect(err).ToNot(HaveOccurred())
			Eventually(receiverServer.envelopes).Should(HaveLen(10))
		})

		It("forwards the request as an egress batch request", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			egressReq := &loggregator_v2.EgressRequest{
				ShardId: "a-shard-id",
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: "a-source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
				UsePreferredTags: true,
			}
			err := server.Receiver(egressReq, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var egressBatchReq *loggregator_v2.EgressBatchRequest
			Eventually(receiver.requests).Should(Receive(&egressBatchReq))
			Expect(egressBatchReq.GetShardId()).To(Equal(egressReq.GetShardId()))
			Expect(egressBatchReq.GetSelectors()).To(Equal(egressReq.GetSelectors()))
			Expect(egressBatchReq.GetUsePreferredTags()).To(Equal(egressReq.GetUsePreferredTags()))
		})

		It("ignores the legacy selector if both old and new selectors are used", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{
				LegacySelector: &loggregator_v2.Selector{
					SourceId: "source-id",
				},
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: "source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
					{
						SourceId: "other-source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var req *loggregator_v2.EgressBatchRequest
			Expect(receiver.requests).Should(Receive(&req))
			Expect(req.Selectors).Should(HaveLen(2))

			Expect(req.LegacySelector).To(BeNil())
		})

		It("upgrades LegacySelector to Selector if no Selectors are present", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.Receiver(&loggregator_v2.EgressRequest{
				LegacySelector: &loggregator_v2.Selector{
					SourceId: "legacy-source-id",
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			}, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var req *loggregator_v2.EgressBatchRequest
			Expect(receiver.requests).Should(Receive(&req))
			Expect(req.Selectors).Should(HaveLen(1))

			Expect(req.LegacySelector).To(BeNil())
			Expect(req.Selectors[0].SourceId).To(Equal("legacy-source-id"))
		})

		It("closes the receiver when the context is canceled", func() {
			receiverServer := newSpyReceiverServer(nil)
			receiver := newSpyReceiver(1000000000)
			ctx, cancel := context.WithCancel(context.TODO())
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				ctx,
				1,
				time.Nanosecond,
			)

			go func() {
				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Expect(err).ToNot(HaveOccurred())
			}()

			cancel()

			var rxCtx context.Context
			Eventually(receiver.ctx).Should(Receive(&rxCtx))
			Eventually(rxCtx.Done).Should(BeClosed())
		})

		It("cancels the context when Receiver exits", func() {
			receiverServer := newSpyReceiverServer(errors.New("Oh no!"))
			receiver := newSpyReceiver(100000000)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			go server.Receiver(&loggregator_v2.EgressRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			var ctx context.Context
			Eventually(receiver.ctx).Should(Receive(&ctx))
			Eventually(ctx.Done()).Should(BeClosed())
		})

		Describe("Preferred Tags option", func() {
			var (
				receiverServer *spyReceiverServer
				receiver       *spyReceiver
				server         *egress.Server
			)

			BeforeEach(func() {
				receiverServer = newSpyReceiverServer(nil)
				receiver = newSpyReceiver(10)
				receiver.envelope = &loggregator_v2.Envelope{
					Tags: map[string]string{
						"a": "value-a",
					},
					DeprecatedTags: map[string]*loggregator_v2.Value{
						"b": {
							Data: &loggregator_v2.Value_Decimal{
								Decimal: 0.8,
							},
						},
						"c": {
							Data: &loggregator_v2.Value_Integer{
								Integer: 18,
							},
						},
						"d": {
							Data: &loggregator_v2.Value_Text{
								Text: "value-d",
							},
						},
					},
				}
				server = egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)
			})

			It("sends deprecated tags", func() {
				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
					UsePreferredTags: false,
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetTags()).To(BeEmpty())
				Expect(e.GetDeprecatedTags()).To(HaveLen(4))

				tags := e.GetDeprecatedTags()
				Expect(tags["a"].GetText()).To(Equal("value-a"))
				Expect(tags["b"].GetDecimal()).To(Equal(0.8))
				Expect(tags["c"].GetInteger()).To(Equal(int64(18)))
				Expect(tags["d"].GetText()).To(Equal("value-d"))
			})

			It("sends preferred tags", func() {
				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
					UsePreferredTags: true,
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetDeprecatedTags()).To(BeEmpty())
				Expect(e.GetTags()).To(HaveLen(4))

				tags := e.GetTags()
				Expect(tags["a"]).To(Equal("value-a"))
				Expect(tags["b"]).To(Equal("0.8"))
				Expect(tags["c"]).To(Equal("18"))
				Expect(tags["d"]).To(Equal("value-d"))
			})

			It("works if deprecated tags is nil when requesting preferred tags", func() {
				receiver.envelope = &loggregator_v2.Envelope{
					Tags: nil,
					DeprecatedTags: map[string]*loggregator_v2.Value{
						"a": {
							Data: &loggregator_v2.Value_Text{
								Text: "value-a",
							},
						},
					},
				}

				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
					UsePreferredTags: true,
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetTags()).To(HaveLen(1))
				Expect(e.GetDeprecatedTags()).To(BeEmpty())
			})

			It("works if tags is nil when requesting deprecated tags", func() {
				receiver.envelope = &loggregator_v2.Envelope{
					DeprecatedTags: nil,
					Tags: map[string]string{
						"a": "value-a",
					},
				}

				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
					UsePreferredTags: false,
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetTags()).To(BeEmpty())
				Expect(e.GetDeprecatedTags()).To(HaveLen(1))
			})
		})

		Describe("Metrics", func() {
			It("emits 'egress' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyReceiverServer(nil)
				receiver := newSpyReceiver(10)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)

				err := server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(BeNumerically("==", 10))
			})

			It("emits 'dropped' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyReceiverServer(nil)
				receiverServer.wait = make(chan struct{})
				defer receiverServer.stopWait()

				receiver := newSpyReceiver(1000000)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("dropped")
				}, 3).Should(BeNumerically(">", 100))
			})

			It("emits 'rejected_streams' metric for each rejected streams", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyReceiverServer(nil)
				receiverServer.wait = make(chan struct{})
				defer receiverServer.stopWait()

				receiver := newSpyReceiver(1)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
					egress.WithMaxStreams(0),
				)

				go server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Eventually(func() uint64 {
					return metricClient.GetDelta("rejected_streams")
				}).Should(Equal(uint64(1)))
			})

			It("emits 'subscriptions' metric for each stream", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyReceiverServer(nil)
				receiver := newSpyReceiver(1000000000)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Eventually(func() float64 {
					return metricClient.GetValue("subscriptions")
				}).Should(Equal(1.0))

				receiver.stop()

				Eventually(func() float64 {
					return metricClient.GetValue("subscriptions")
				}).Should(Equal(0.0))
			})
		})

		Describe("health monitoring", func() {
			It("increments and decrements subscription count", func() {
				receiverServer := newSpyReceiverServer(nil)
				receiver := newSpyReceiver(1000000000)

				health := newSpyHealthRegistrar()
				server := egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					health,
					context.TODO(),
					1,
					time.Nanosecond,
				)
				go server.Receiver(&loggregator_v2.EgressRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(1.0))

				receiver.stop()

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(0.0))
			})
		})
	})

	Describe("BatchedReceiver()", func() {
		It("errors when streams exceeds maxStreams", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			server := egress.NewServer(
				&stubReceiver{},
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
				egress.WithMaxStreams(0),
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			Expect(err).To(MatchError(
				status.Errorf(
					codes.ResourceExhausted,
					"unable to create stream, max egress streams reached: 0",
				),
			))
		})

		It("errors when the sender cannot send the envelope", func() {
			receiverServer := newSpyBatchedReceiverServer(errors.New("Oh No!"))
			server := egress.NewServer(
				&stubReceiver{},
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			Expect(err).To(Equal(io.ErrUnexpectedEOF))
		})

		It("errors when there aren't any Message selectors", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			server := egress.NewServer(
				&stubReceiver{},
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{}, receiverServer)

			Expect(err).To(HaveOccurred())

			err = server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{{}},
			}, receiverServer)

			Expect(err).To(HaveOccurred())
		})

		It("streams data when there are envelopes", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			server := egress.NewServer(
				newSpyReceiver(10),
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				10,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			Expect(err).ToNot(HaveOccurred())
			Eventually(receiverServer.envelopes).Should(HaveLen(10))
		})

		It("ignores the legacy selector if both old and new selectors are used", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				LegacySelector: &loggregator_v2.Selector{
					SourceId: "source-id",
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: "source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
					{
						SourceId: "other-source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var req *loggregator_v2.EgressBatchRequest
			Expect(receiver.requests).Should(Receive(&req))
			Expect(req.Selectors).Should(HaveLen(2))

			Expect(req.LegacySelector).To(BeNil())
		})

		It("upgrades LegacySelector to Selector if no Selectors are present for batching", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				LegacySelector: &loggregator_v2.Selector{
					SourceId: "legacy-source-id",
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			}, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			var req *loggregator_v2.EgressBatchRequest
			Expect(receiver.requests).Should(Receive(&req))
			Expect(req.Selectors).Should(HaveLen(1))

			Expect(req.LegacySelector).To(BeNil())
			Expect(req.Selectors[0].SourceId).To(Equal("legacy-source-id"))
		})

		It("passes the egress batch request through", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			receiver := newSpyReceiver(10)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)

			expectedReq := &loggregator_v2.EgressBatchRequest{
				ShardId: "a-shard-id",
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: "a-source-id",
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
				UsePreferredTags: true,
			}
			err := server.BatchedReceiver(expectedReq, receiverServer)
			Expect(err).ToNot(HaveOccurred())

			Eventually(receiver.requests).Should(Receive(Equal(expectedReq)))
		})

		It("closes the receiver when the context is canceled", func() {
			receiverServer := newSpyBatchedReceiverServer(nil)
			receiver := newSpyReceiver(1000000000)

			ctx, cancel := context.WithCancel(context.TODO())
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				ctx,
				1,
				time.Nanosecond,
			)

			go func() {
				err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())
			}()

			cancel()

			var rxCtx context.Context
			Eventually(receiver.ctx).Should(Receive(&rxCtx))
			Eventually(rxCtx.Done).Should(BeClosed())
		})

		It("cancels the context when Receiver exits", func() {
			receiverServer := newSpyBatchedReceiverServer(errors.New("Oh no!"))
			receiver := newSpyReceiver(100000000)
			server := egress.NewServer(
				receiver,
				testhelper.NewMetricClient(),
				newSpyHealthRegistrar(),
				context.TODO(),
				1,
				time.Nanosecond,
			)
			go server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			}, receiverServer)

			var ctx context.Context
			Eventually(receiver.ctx).Should(Receive(&ctx))
			Eventually(ctx.Done()).Should(BeClosed())
		})

		Describe("Preferred Tags option", func() {
			var (
				receiverServer *spyBatchedReceiverServer
				receiver       *spyReceiver
				server         *egress.Server
			)

			BeforeEach(func() {
				receiverServer = newSpyBatchedReceiverServer(nil)
				receiver = newSpyReceiver(10)
				receiver.envelope = &loggregator_v2.Envelope{
					Tags: map[string]string{
						"a": "value-a",
					},
					DeprecatedTags: map[string]*loggregator_v2.Value{
						"b": {
							Data: &loggregator_v2.Value_Decimal{
								Decimal: 0.8,
							},
						},
						"c": {
							Data: &loggregator_v2.Value_Integer{
								Integer: 18,
							},
						},
						"d": {
							Data: &loggregator_v2.Value_Text{
								Text: "value-d",
							},
						},
					},
				}
				server = egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)
			})

			It("sends deprecated tags", func() {
				err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
					UsePreferredTags: false,
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetTags()).To(BeEmpty())
				Expect(e.GetDeprecatedTags()).To(HaveLen(4))

				tags := e.GetDeprecatedTags()
				Expect(tags["a"].GetText()).To(Equal("value-a"))
				Expect(tags["b"].GetDecimal()).To(Equal(0.8))
				Expect(tags["c"].GetInteger()).To(Equal(int64(18)))
				Expect(tags["d"].GetText()).To(Equal("value-d"))
			})

			It("sends preferred tags", func() {
				err := server.BatchedReceiver(&loggregator_v2.EgressBatchRequest{
					UsePreferredTags: true,
					Selectors: []*loggregator_v2.Selector{
						{
							Message: &loggregator_v2.Selector_Log{
								Log: &loggregator_v2.LogSelector{},
							},
						},
					},
				}, receiverServer)
				Expect(err).ToNot(HaveOccurred())

				var e *loggregator_v2.Envelope
				Eventually(receiverServer.envelopes).Should(Receive(&e))
				Expect(e.GetDeprecatedTags()).To(BeEmpty())
				Expect(e.GetTags()).To(HaveLen(4))

				tags := e.GetTags()
				Expect(tags["a"]).To(Equal("value-a"))
				Expect(tags["b"]).To(Equal("0.8"))
				Expect(tags["c"]).To(Equal("18"))
				Expect(tags["d"]).To(Equal("value-d"))
			})
		})

		Describe("Metrics", func() {
			It("emits 'egress' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiver := newSpyReceiver(10)
				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					10,
					time.Second,
				)

				err := server.BatchedReceiver(
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					},
					newSpyBatchedReceiverServer(nil),
				)

				Expect(err).ToNot(HaveOccurred())
				Eventually(func() uint64 {
					return metricClient.GetDelta("egress")
				}).Should(BeNumerically("==", 10))
			})

			It("emits 'dropped' metric for each envelope", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyBatchedReceiverServer(nil)
				receiverServer.delay = 100 * time.Millisecond
				receiver := newSpyReceiver(1000000)

				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
				)
				go server.BatchedReceiver(
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					}, receiverServer,
				)

				Eventually(func() uint64 {
					return metricClient.GetDelta("dropped")
				}, 3).Should(BeNumerically(">", 100))
			})

			It("emits 'rejected_streams' metric when streams are rejected", func() {
				metricClient := testhelper.NewMetricClient()
				receiverServer := newSpyBatchedReceiverServer(nil)
				receiver := newSpyReceiver(1)

				server := egress.NewServer(
					receiver,
					metricClient,
					newSpyHealthRegistrar(),
					context.TODO(),
					1,
					time.Nanosecond,
					egress.WithMaxStreams(0),
				)
				go server.BatchedReceiver(
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					}, receiverServer,
				)

				Eventually(func() uint64 {
					return metricClient.GetDelta("rejected_streams")
				}).Should(Equal(uint64(1)))
			})

			It("emits 'subscriptions' metric for each stream", func() {
				metricClient := testhelper.NewMetricClient()
				receiver := newSpyReceiver(1000000000)
				health := newSpyHealthRegistrar()
				server := egress.NewServer(
					receiver,
					metricClient,
					health,
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.BatchedReceiver(
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					},
					newSpyBatchedReceiverServer(nil),
				)

				Eventually(func() float64 {
					return metricClient.GetValue("subscriptions")
				}).Should(Equal(1.0))

				receiver.stop()

				Eventually(func() float64 {
					return metricClient.GetValue("subscriptions")
				}).Should(Equal(0.0))
			})
		})

		Describe("health monitoring", func() {
			It("increments and decrements subscription count", func() {
				receiver := newSpyReceiver(1000000000)
				health := newSpyHealthRegistrar()
				server := egress.NewServer(
					receiver,
					testhelper.NewMetricClient(),
					health,
					context.TODO(),
					1,
					time.Nanosecond,
				)

				go server.BatchedReceiver(
					&loggregator_v2.EgressBatchRequest{
						Selectors: []*loggregator_v2.Selector{
							{
								Message: &loggregator_v2.Selector_Log{
									Log: &loggregator_v2.LogSelector{},
								},
							},
						},
					},
					newSpyBatchedReceiverServer(nil),
				)

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(1.0))

				receiver.stop()

				Eventually(func() float64 {
					return health.Get("subscriptionCount")
				}).Should(Equal(0.0))
			})
		})
	})
})

type spyReceiverServer struct {
	err       error
	wait      chan struct{}
	envelopes chan *loggregator_v2.Envelope

	grpc.ServerStream
}

func newSpyReceiverServer(err error) *spyReceiverServer {
	return &spyReceiverServer{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
		err:       err,
	}
}

func (*spyReceiverServer) Context() context.Context {
	return context.Background()
}

func (s *spyReceiverServer) Send(e *loggregator_v2.Envelope) error {
	if s.wait != nil {
		<-s.wait
		return nil
	}

	select {
	case s.envelopes <- e:
	default:
	}

	return s.err
}

func (s *spyReceiverServer) stopWait() {
	close(s.wait)
}

type spyBatchedReceiverServer struct {
	err       error
	envelopes chan *loggregator_v2.Envelope
	delay     time.Duration

	grpc.ServerStream
}

func newSpyBatchedReceiverServer(err error) *spyBatchedReceiverServer {
	return &spyBatchedReceiverServer{
		envelopes: make(chan *loggregator_v2.Envelope, 1000),
		err:       err,
	}
}

func (*spyBatchedReceiverServer) Context() context.Context {
	return context.Background()
}

func (s *spyBatchedReceiverServer) Send(b *loggregator_v2.EnvelopeBatch) error {
	for _, e := range b.GetBatch() {
		select {
		case s.envelopes <- e:
			time.Sleep(s.delay)
		default:
		}
	}

	return s.err
}

type spyReceiver struct {
	envelope       *loggregator_v2.Envelope
	envelopeRepeat int

	stopCh   chan struct{}
	ctx      chan context.Context
	requests chan *loggregator_v2.EgressBatchRequest
}

func newSpyReceiver(envelopeCount int) *spyReceiver {
	return &spyReceiver{
		envelope:       &loggregator_v2.Envelope{},
		envelopeRepeat: envelopeCount,
		stopCh:         make(chan struct{}),
		ctx:            make(chan context.Context, 1),
		requests:       make(chan *loggregator_v2.EgressBatchRequest, 100),
	}
}

func (s *spyReceiver) Subscribe(ctx context.Context, req *loggregator_v2.EgressBatchRequest) (func() (*loggregator_v2.Envelope, error), error) {
	s.ctx <- ctx
	s.requests <- req

	return func() (*loggregator_v2.Envelope, error) {
		if s.envelopeRepeat > 0 {
			select {
			case <-s.stopCh:
				return nil, io.EOF
			default:
				s.envelopeRepeat--
				return s.envelope, nil
			}
		}

		return nil, errors.New("Oh no!")
	}, nil
}

type stubReceiver struct{}

func (s *stubReceiver) Subscribe(ctx context.Context, req *loggregator_v2.EgressBatchRequest) (func() (*loggregator_v2.Envelope, error), error) {
	rx := func() (*loggregator_v2.Envelope, error) {
		return &loggregator_v2.Envelope{}, nil
	}
	return rx, nil
}

func (s *spyReceiver) stop() {
	close(s.stopCh)
}

type SpyHealthRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
}

func newSpyHealthRegistrar() *SpyHealthRegistrar {
	return &SpyHealthRegistrar{
		values: make(map[string]float64),
	}
}

func (s *SpyHealthRegistrar) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]++
}

func (s *SpyHealthRegistrar) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]--
}

func (s *SpyHealthRegistrar) Get(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}
