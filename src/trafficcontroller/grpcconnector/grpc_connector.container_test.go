package grpcconnector_test

import (
	"fmt"
	"plumbing"
	"trafficcontroller/grpcconnector"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//
// TODO Rename file
//

var _ = Describe("GrpcContainerMetrics", func() {

	var (
		mockReceiveFetcher *mockReceiveFetcher
		mockMetricBatcher  *mockMetaMetricBatcher
		mockBatchChainer   *mockBatchCounterChainer

		connector *grpcconnector.GrpcConnector
	)

	BeforeEach(func() {
		mockReceiveFetcher = newMockReceiveFetcher()
		mockBatchChainer = newMockBatchCounterChainer()
		mockMetricBatcher = newMockMetaMetricBatcher()
		connector = grpcconnector.New(mockReceiveFetcher, mockMetricBatcher)
	})

	Context("when the fetcher returns metrics", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret1 <- nil
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret0 <- []*plumbing.ContainerMetricsResponse{
				{
					Payload: [][]byte{
						[]byte("foo"),
						[]byte("bar"),
						[]byte("baz"),
					},
				},
				{
					Payload: [][]byte{
						[]byte("bacon"),
						[]byte("eggs"),
					},
				},
			}
		})

		It("returns latest container metrics from all dopplers", func() {
			resp, err := connector.ContainerMetrics(context.Background(), &plumbing.ContainerMetricsRequest{AppID: "AppID"})

			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			Expect(resp.Payload).To(ConsistOf(
				[]byte("foo"),
				[]byte("bar"),
				[]byte("baz"),
				[]byte("bacon"),
				[]byte("eggs"),
			))
		})
	})

	Context("when the fetcher does not return any metrics", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret0 <- nil
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret1 <- nil
		})

		It("returns an empty payload", func() {
			resp, err := connector.ContainerMetrics(context.Background(), &plumbing.ContainerMetricsRequest{AppID: "AppID"})
			Expect(err).ToNot(HaveOccurred())

			Expect(resp).ToNot(BeNil())
			Expect(resp.Payload).To(HaveLen(0))
		})
	})

	Context("when the fetcher returns an error", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret0 <- nil
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret1 <- fmt.Errorf("some-error")
		})

		It("returns an error", func() {
			_, err := connector.ContainerMetrics(context.Background(), &plumbing.ContainerMetricsRequest{AppID: "AppID"})
			Expect(err).To(HaveOccurred())
		})
	})
})
