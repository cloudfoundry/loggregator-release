package metricemitter

import (
	"context"
	v2 "plumbing/v2"
	"time"

	"google.golang.org/grpc"
)

type client struct {
	ingressClient v2.IngressClient
	pulseInterval time.Duration
	dialOpts      []grpc.DialOption
	sourceID      string
	tags          map[string]string
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
		tags: make(map[string]string),
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

func (c *client) NewMetric(name string, opts ...MetricOption) *metric {
	opts = append(opts, WithTags(c.tags))
	m := newMetric(name, opts...)
	go c.pulse(m)

	return m
}

func (c *client) pulse(m *metric) {
	senderClient, _ := c.ingressClient.Sender(context.Background())

	for range time.Tick(c.pulseInterval) {
		envelope := &v2.Envelope{
			SourceId:  c.sourceID,
			Timestamp: time.Now().UnixNano(),
			Message: &v2.Envelope_Counter{
				Counter: &v2.Counter{
					Name: m.name,
					Value: &v2.Counter_Delta{
						Delta: m.GetDelta(),
					},
				},
			},
			Tags: m.tags,
		}

		senderClient.Send(envelope)
	}
}
