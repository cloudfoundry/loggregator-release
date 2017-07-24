package egress

import (
	"errors"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"golang.org/x/net/context"
)

type ContainerMetricFetcher interface {
	ContainerMetrics(ctx context.Context, sourceId string, usePreferredTags bool) ([]*v2.Envelope, error)
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
	if req.SourceId == "" {
		return nil, errors.New("source_id is required")
	}

	results, err := s.fetcher.ContainerMetrics(ctx, req.SourceId, req.UsePreferredTags)
	if err != nil {
		return nil, err
	}

	return &v2.QueryResponse{
		Envelopes: results,
	}, nil
}
