package grpcconnector_test

import (
	"plumbing"
	"trafficcontroller/grpcconnector"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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

	It("returns latest container metrics from all dopplers", func() {
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
