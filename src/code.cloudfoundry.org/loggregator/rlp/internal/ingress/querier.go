package ingress

import (
	"log"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"golang.org/x/net/context"
)

type ContainerMetricFetcher interface {
	ContainerMetrics(ctx context.Context, appID string) [][]byte
}

type Querier struct {
	fetcher   ContainerMetricFetcher
	converter EnvelopeConverter
}

func NewQuerier(c EnvelopeConverter, f ContainerMetricFetcher) *Querier {
	return &Querier{
		fetcher:   f,
		converter: c,
	}
}

func (q *Querier) ContainerMetrics(ctx context.Context, sourceId string, usePreferredTags bool) ([]*v2.Envelope, error) {
	results := q.fetcher.ContainerMetrics(ctx, sourceId)

	var v2Envs []*v2.Envelope
	for _, envBytes := range results {
		v2e, err := q.converter.Convert(envBytes, usePreferredTags)
		if err != nil {
			log.Printf("Invalid container envelope: %s", err)
			continue
		}

		v2Envs = append(v2Envs, v2e)
	}

	return v2Envs, nil
}
