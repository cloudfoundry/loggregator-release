//go:generate hel
package grpcconnector_test

import (
	"fmt"
	"plumbing"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GrpcConnector", func() {
	var (
		mockReceiveFetcher *mockReceiveFetcher
		mockReceiverA      *mockReceiver
		mockReceiverB      *mockReceiver

		connector *grpcconnector.GrpcConnector
	)

	var fetchPayloads = func(count int, client grpcconnector.Receiver) (chan []byte, chan error) {
		c := make(chan []byte, 100)
		e := make(chan error, 100)
		go func() {
			for i := 0; i < count; i++ {
				resp, err := client.Recv()
				if err != nil {
					e <- err
					return
				}

				c <- resp.Payload
			}
		}()
		return c, e
	}

	var channelToSlice = func(count int, c chan []byte) [][]byte {
		var results [][]byte
		for i := 0; i < count; i++ {
			var r []byte
			Eventually(c).Should(Receive(&r))
			results = append(results, r)
		}

		return results
	}

	BeforeEach(func() {
		mockReceiveFetcher = newMockReceiveFetcher()
		mockReceiverA = newMockReceiver()
		mockReceiverB = newMockReceiver()
		connector = grpcconnector.New(mockReceiveFetcher)

		mockReceiveFetcher.FetchStreamOutput.Ret0 <- []grpcconnector.Receiver{mockReceiverA, mockReceiverB}
		mockReceiveFetcher.FetchFirehoseOutput.Ret0 <- []grpcconnector.Receiver{mockReceiverA, mockReceiverB}
	})

	Describe("Stream", func() {
		Context("fetcher does not return an error", func() {
			BeforeEach(func() {
				close(mockReceiveFetcher.FetchStreamOutput.Ret1)
			})

			Context("receivers don't return an error", func() {
				BeforeEach(func() {
					close(mockReceiverA.RecvOutput.Ret1)
					close(mockReceiverB.RecvOutput.Ret1)
				})

				Context("when data has been written from server", func() {
					BeforeEach(func() {
						mockReceiverA.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-a")}
						mockReceiverB.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-b")}
					})

					It("passes through the arguments", func() {
						ctx := context.Background()
						req := &plumbing.StreamRequest{"AppID"}
						opt := grpc.Header(nil)
						opts := []grpc.CallOption{opt}
						connector.Stream(ctx, req, opts...)
						Expect(mockReceiveFetcher.FetchStreamInput).To(BeCalled(With(ctx, req, opts)))
					})

					It("reads from both receivers", func() {
						client, err := connector.Stream(context.Background(), &plumbing.StreamRequest{"AppID"})
						Expect(err).ToNot(HaveOccurred())

						payloads, _ := fetchPayloads(2, client)
						Eventually(payloads).Should(HaveLen(2))

						data := channelToSlice(2, payloads)
						Expect(data).To(ContainElement([]byte("some-data-a")))
						Expect(data).To(ContainElement([]byte("some-data-b")))
					})
				})
			})

			Context("receiver returns an error", func() {
				BeforeEach(func() {
					mockReceiverA.RecvOutput.Ret0 <- nil
					mockReceiverA.RecvOutput.Ret1 <- fmt.Errorf("some-error")

					mockReceiverB.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-b")}
					mockReceiverB.RecvOutput.Ret1 <- nil
				})

				It("it returns an error", func() {
					client, err := connector.Stream(context.Background(), &plumbing.StreamRequest{"AppID"})
					Expect(err).ToNot(HaveOccurred())

					_, errs := fetchPayloads(2, client)
					Eventually(errs).ShouldNot(BeEmpty())
				})
			})
		})

		Context("fetcher returns an error", func() {
			BeforeEach(func() {
				close(mockReceiveFetcher.FetchStreamOutput.Ret0)
				mockReceiveFetcher.FetchStreamOutput.Ret1 <- fmt.Errorf("some-error")
			})

			It("returns an error", func() {
				_, err := connector.Stream(context.Background(), &plumbing.StreamRequest{"AppID"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Firehose", func() {
		Context("fetcher does not return an error", func() {
			BeforeEach(func() {
				close(mockReceiveFetcher.FetchFirehoseOutput.Ret1)
			})

			Context("receivers don't return an error", func() {
				BeforeEach(func() {
					close(mockReceiverA.RecvOutput.Ret1)
					close(mockReceiverB.RecvOutput.Ret1)
				})

				Context("when data has been written from server", func() {
					BeforeEach(func() {
						mockReceiverA.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-a")}
						mockReceiverB.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-b")}
					})

					It("passes through the arguments", func(done Done) {
						defer close(done)
						ctx := context.Background()
						req := &plumbing.FirehoseRequest{"AppID"}
						opt := grpc.Header(nil)
						opts := []grpc.CallOption{opt}
						connector.Firehose(ctx, req, opts...)
						Expect(mockReceiveFetcher.FetchFirehoseInput).To(BeCalled(With(ctx, req, opts)))
					})

					It("reads from both receivers", func() {
						client, err := connector.Firehose(context.Background(), &plumbing.FirehoseRequest{"AppID"})
						Expect(err).ToNot(HaveOccurred())

						payloads, _ := fetchPayloads(2, client)
						Eventually(payloads).Should(HaveLen(2))

						data := channelToSlice(2, payloads)
						Expect(data).To(ContainElement([]byte("some-data-a")))
						Expect(data).To(ContainElement([]byte("some-data-b")))
					})
				})
			})

			Context("receiver returns an error", func() {
				BeforeEach(func() {
					mockReceiverA.RecvOutput.Ret0 <- nil
					mockReceiverA.RecvOutput.Ret1 <- fmt.Errorf("some-error")

					mockReceiverB.RecvOutput.Ret0 <- &plumbing.Response{[]byte("some-data-b")}
					mockReceiverB.RecvOutput.Ret1 <- nil
				})

				It("it returns an error", func() {
					client, err := connector.Firehose(context.Background(), &plumbing.FirehoseRequest{"AppID"})
					Expect(err).ToNot(HaveOccurred())

					_, errs := fetchPayloads(2, client)
					Eventually(errs).ShouldNot(BeEmpty())
				})
			})
		})

		Context("fetcher returns an error", func() {
			BeforeEach(func() {
				close(mockReceiveFetcher.FetchFirehoseOutput.Ret0)
				mockReceiveFetcher.FetchFirehoseOutput.Ret1 <- fmt.Errorf("some-error")
			})

			It("returns an error", func() {
				_, err := connector.Firehose(context.Background(), &plumbing.FirehoseRequest{"AppID"})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
