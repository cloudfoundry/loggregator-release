package grpcconnector

import (
	"plumbing"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Receiver interface {
	Recv() (*plumbing.Response, error)
}

type ReceiveFetcher interface {
	FetchStream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) ([]Receiver, error)
	FetchFirehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) ([]Receiver, error)
	FetchContainerMetrics(ctx context.Context, in *plumbing.ContainerMetricsRequest) ([]*plumbing.ContainerMetricsResponse, error)
	FetchRecentLogs(ctx context.Context, in *plumbing.RecentLogsRequest) ([]*plumbing.RecentLogsResponse, error)
}

type MetaMetricBatcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type GRPCConnector struct {
	fetcher ReceiveFetcher
	batcher MetaMetricBatcher
}

type grpcConnInfo struct {
	dopplerClient plumbing.DopplerClient
	conn          *grpc.ClientConn
}

func New(fetcher ReceiveFetcher, batcher MetaMetricBatcher) *GRPCConnector {
	return &GRPCConnector{
		fetcher: fetcher,
		batcher: batcher,
	}
}

func (g *GRPCConnector) Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchStream(ctx, in, opts...)
	return startCombiner(rxs, g.batcher), err
}

func (g *GRPCConnector) Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchFirehose(ctx, in, opts...)
	return startCombiner(rxs, g.batcher), err
}

func (g *GRPCConnector) RecentLogs(ctx context.Context, in *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	responses, err := g.fetcher.FetchRecentLogs(ctx, in)

	if err != nil {
		return nil, err
	}

	resp := new(plumbing.RecentLogsResponse)
	for _, response := range responses {
		resp.Payload = append(resp.Payload, response.Payload...)
	}

	return resp, nil
}

func (g *GRPCConnector) ContainerMetrics(ctx context.Context, in *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	responses, err := g.fetcher.FetchContainerMetrics(ctx, in)

	if err != nil {
		return nil, err
	}

	resp := new(plumbing.ContainerMetricsResponse)
	for _, response := range responses {
		resp.Payload = append(resp.Payload, response.Payload...)
	}

	resp.Payload = deDupe(resp.Payload)

	return resp, nil
}

func deDupe(input [][]byte) [][]byte {
	messages := make(map[int32]*events.Envelope)

	for _, message := range input {
		var envelope events.Envelope
		proto.Unmarshal(message, &envelope)
		cm := envelope.GetContainerMetric()

		oldEnvelope, ok := messages[cm.GetInstanceIndex()]
		if !ok || oldEnvelope.GetTimestamp() < envelope.GetTimestamp() {
			messages[cm.GetInstanceIndex()] = &envelope
		}
	}

	output := make([][]byte, 0, len(messages))

	for _, envelope := range messages {
		bytes, _ := proto.Marshal(envelope)
		output = append(output, bytes)
	}
	return output
}
