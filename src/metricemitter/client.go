package metricemitter

import (
	"context"
	v2 "plumbing/v2"
	"time"

	"google.golang.org/grpc"
)

type MetricClient interface {
	NewCounterMetric(name string, opts ...MetricOption) *CounterMetric
}

type client struct {
	ingressClient v2.IngressClient
	pulseInterval time.Duration
	dialOpts      []grpc.DialOption
	sourceID      string
	tags          map[string]string
}

type sendable interface {
	WithEnvelope(func(*v2.Envelope) error) error
}

type ClientOption func(*client)

func WithGRPCDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *client) {
		c.dialOpts = opts
	}
}

func WithPulseInterval(d time.Duration) ClientOption {
	return func(c *client) {
		c.pulseInterval = d
	}
}

func WithSourceID(s string) ClientOption {
	return func(c *client) {
		c.sourceID = s
	}
}

func WithOrigin(name string) ClientOption {
	return func(c *client) {
		c.tags["origin"] = name
	}
}

func WithDeployment(deployment, job, index string) ClientOption {
	return func(c *client) {
		c.tags["deployment"] = deployment
		c.tags["job"] = job
		c.tags["index"] = index
	}
}

func NewClient(addr string, opts ...ClientOption) (*client, error) {
	client := &client{
		tags:          make(map[string]string),
		pulseInterval: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(client)
	}

	conn, err := grpc.Dial(addr, client.dialOpts...)
	if err != nil {
		return nil, err
	}

	client.ingressClient = v2.NewIngressClient(conn)

	return client, nil
}

func (c *client) NewCounterMetric(name string, opts ...MetricOption) *CounterMetric {
	opts = append(opts, WithTags(c.tags))
	m := NewCounterMetric(name, c.sourceID, opts...)
	go c.pulse(m)

	return m
}

func (c *client) pulse(s sendable) {
	var senderClient v2.Ingress_SenderClient
	for range time.Tick(c.pulseInterval) {
		if senderClient == nil {
			var err error
			senderClient, err = c.ingressClient.Sender(context.Background())
			if err != nil {
				continue
			}
		}

		err := s.WithEnvelope(func(env *v2.Envelope) error {
			return senderClient.Send(env)
		})

		if err != nil {
			senderClient = nil
		}
	}
}
