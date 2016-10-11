package grpcconnector

import (
	"plumbing"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

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
}

type MetaMetricBatcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type GrpcConnector struct {
	fetcher ReceiveFetcher
	batcher MetaMetricBatcher
}

type grpcConnInfo struct {
	dopplerClient plumbing.DopplerClient
	conn          *grpc.ClientConn
}

func New(fetcher ReceiveFetcher, batcher MetaMetricBatcher) *GrpcConnector {
	return &GrpcConnector{
		fetcher: fetcher,
		batcher: batcher,
	}
}

func (g *GrpcConnector) Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchStream(ctx, in, opts...)
	return startCombiner(rxs, g.batcher), err
}

func (g *GrpcConnector) Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchFirehose(ctx, in, opts...)
	return startCombiner(rxs, g.batcher), err
}

func (g *GrpcConnector) ContainerMetrics(ctx context.Context, in *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	responses, err := g.fetcher.FetchContainerMetrics(ctx, in)

	if err != nil {
		return nil, err
	}

	resp := new(plumbing.ContainerMetricsResponse)
	if len(responses) == 0 {
		return resp, nil
	}

	for _, response := range responses {
		resp.Payload = append(resp.Payload, response.Payload...)
	}

	return resp, nil
}
