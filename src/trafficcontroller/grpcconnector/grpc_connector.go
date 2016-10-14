package grpcconnector

import (
	"plumbing"
	"time"

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
}

type MetaMetricBatcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type GrpcConnector struct {
	fetcher    ReceiveFetcher
	batcher    MetaMetricBatcher
	killAfter  time.Duration
	bufferSize int
}

type grpcConnInfo struct {
	dopplerClient plumbing.DopplerClient
	conn          *grpc.ClientConn
}

func New(fetcher ReceiveFetcher, batcher MetaMetricBatcher, killAfter time.Duration, bufferSize int) *GrpcConnector {
	return &GrpcConnector{
		fetcher:    fetcher,
		batcher:    batcher,
		killAfter:  killAfter,
		bufferSize: bufferSize,
	}
}

func (g *GrpcConnector) Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchStream(ctx, in, opts...)
	return startCombiner(rxs, g.batcher, g.killAfter, g.bufferSize), err
}

func (g *GrpcConnector) Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchFirehose(ctx, in, opts...)
	return startCombiner(rxs, g.batcher, g.killAfter, g.bufferSize), err
}
