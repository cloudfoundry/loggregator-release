package grpcconnector

import (
	"plumbing"

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

type GrpcConnector struct {
	fetcher ReceiveFetcher
}

type grpcConnInfo struct {
	dopplerClient plumbing.DopplerClient
	conn          *grpc.ClientConn
}

func New(fetcher ReceiveFetcher) *GrpcConnector {
	return &GrpcConnector{
		fetcher: fetcher,
	}
}

func (g *GrpcConnector) Stream(ctx context.Context, in *plumbing.StreamRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchStream(ctx, in, opts...)
	return startCombiner(rxs), err
}

func (g *GrpcConnector) Firehose(ctx context.Context, in *plumbing.FirehoseRequest, opts ...grpc.CallOption) (Receiver, error) {
	rxs, err := g.fetcher.FetchFirehose(ctx, in, opts...)
	return startCombiner(rxs), err
}
