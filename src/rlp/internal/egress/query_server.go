package egress

import (
	v2 "plumbing/v2"

	"golang.org/x/net/context"
)

type ContainerMetricFetcher interface {
	ContainerMetrics(ctx context.Context, sourceId string) ([]*v2.Envelope, error)
}

type QueryServer struct {
	fetcher ContainerMetricFetcher
}

func NewQueryServer(f ContainerMetricFetcher) *QueryServer {
	return &QueryServer{
		fetcher: f,
	}
}

func (s *QueryServer) ContainerMetrics(ctx context.Context, req *v2.ContainerMetricRequest) (*v2.QueryResponse, error) {
	results, err := s.fetcher.ContainerMetrics(ctx, req.SourceId)
	if err != nil {
		return nil, err
	}

	return &v2.QueryResponse{
		Envelopes: results,
	}, nil
}
