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
	rxs, err := g.fetcher.FetchContainerMetrics(ctx, in)
	_ = err
	resp := rxs[0]
	for _, rx := range rxs[1:] {
		resp.Payload = append(resp.Payload, rx.Payload...)
	}
	return resp, nil
}
