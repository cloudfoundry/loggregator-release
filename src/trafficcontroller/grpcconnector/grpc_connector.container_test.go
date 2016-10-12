package grpcconnector_test

import (
	"fmt"
	"plumbing"
	"trafficcontroller/grpcconnector"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
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

	Context("when the fetcher returns metrics", func() {
		BeforeEach(func() {
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret1 <- nil
			mockReceiveFetcher.FetchContainerMetricsOutput.Ret0 <- []*plumbing.ContainerMetricsResponse{
				{
					Payload: [][]byte{
						buildContainerMetric(1),
						buildContainerMetric(2),
						buildContainerMetric(3),
					},
				},
				{
					Payload: [][]byte{
						buildContainerMetric(3),
						buildContainerMetric(4),
					},
				},
			}
		})

		It("returns latest container metrics from all dopplers", func() {
			resp, err := connector.ContainerMetrics(context.Background(), &plumbing.ContainerMetricsRequest{AppID: "AppID"})
			Expect(err).ToNot(HaveOccurred())

			Expect(resp).ToNot(BeNil())
			Expect(resp.Payload).To(HaveLen(4))
			Expect(resp.Payload).To(ConsistOf(
				buildContainerMetric(1),
				buildContainerMetric(2),
				buildContainerMetric(3),
				buildContainerMetric(4),
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

func buildContainerMetric(instance int) []byte {
	env := &events.Envelope{
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(10000),
		Origin:    proto.String("some-origin"),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("myApp"),
			InstanceIndex: proto.Int32(int32(instance)),
			CpuPercentage: proto.Float64(73),
			MemoryBytes:   proto.Uint64(2),
			DiskBytes:     proto.Uint64(3),
		},
	}
	bytes, err := proto.Marshal(env)
	Expect(err).ToNot(HaveOccurred())

	return bytes
}
