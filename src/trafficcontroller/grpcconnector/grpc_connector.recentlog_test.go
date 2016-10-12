package grpcconnector_test

import (
	"fmt"
	"plumbing"
	"trafficcontroller/grpcconnector"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCRecentLogs", func() {

	var (
		mockReceiveFetcher *mockReceiveFetcher
		mockMetricBatcher  *mockMetaMetricBatcher
		mockBatchChainer   *mockBatchCounterChainer

		connector *grpcconnector.GRPCConnector
	)

	BeforeEach(func() {
		mockReceiveFetcher = newMockReceiveFetcher()
		mockBatchChainer = newMockBatchCounterChainer()
		mockMetricBatcher = newMockMetaMetricBatcher()
		connector = grpcconnector.New(mockReceiveFetcher, mockMetricBatcher)
	})

	Context("when the fetcher returns recent logs", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchRecentLogsOutput.Ret1 <- nil
			mockReceiveFetcher.FetchRecentLogsOutput.Ret0 <- []*plumbing.RecentLogsResponse{
				{
					Payload: [][]byte{
						[]byte("log1"),
						[]byte("log2"),
						[]byte("log3"),
					},
				},
				{
					Payload: [][]byte{
						[]byte("log4"),
						[]byte("log5"),
					},
				},
			}
		})

		It("returns most recent logs from all dopplers", func() {
			resp, err := connector.RecentLogs(context.Background(), &plumbing.RecentLogsRequest{AppID: "AppID"})
			Expect(err).ToNot(HaveOccurred())

			Expect(resp).ToNot(BeNil())
			Expect(resp.Payload).To(HaveLen(5))
			Expect(resp.Payload).To(ConsistOf(
				[]byte("log1"),
				[]byte("log2"),
				[]byte("log3"),
				[]byte("log4"),
				[]byte("log5"),
			))
		})
	})

	Context("when the fetcher does not return any recent logs", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchRecentLogsOutput.Ret0 <- nil
			mockReceiveFetcher.FetchRecentLogsOutput.Ret1 <- nil
		})

		It("returns an empty payload", func() {
			resp, err := connector.RecentLogs(context.Background(), &plumbing.RecentLogsRequest{AppID: "AppID"})
			Expect(err).ToNot(HaveOccurred())

			Expect(resp).ToNot(BeNil())
			Expect(resp.Payload).To(HaveLen(0))
		})
	})

	Context("when the fetcher returns an error", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchRecentLogsOutput.Ret0 <- nil
			mockReceiveFetcher.FetchRecentLogsOutput.Ret1 <- fmt.Errorf("some-error")
		})

		It("returns an error", func() {
			_, err := connector.RecentLogs(context.Background(), &plumbing.RecentLogsRequest{AppID: "AppID"})
			Expect(err).To(HaveOccurred())
		})
	})
})
