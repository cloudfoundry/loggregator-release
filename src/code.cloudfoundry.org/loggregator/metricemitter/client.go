package metricemitter

import (
	"context"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"google.golang.org/grpc"
)

// Client is used to initialize and emit metrics on a given pulse interval.
type Client struct {
	ingressClient v2.IngressClient
	pulseInterval time.Duration
	dialOpts      []grpc.DialOption
	sourceID      string
	tags          map[string]string
}

type sendable interface {
	WithEnvelope(func(*v2.Envelope) error) error
}

// ClientOption is a function that can be passed into the NewClient for
// optional configuration on the client.
type ClientOption func(*Client)

// WithGRPCDialOptions is a ClientOption that will set the gRPC dial options on
// the Clients IngressClient.
func WithGRPCDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *Client) {
		c.dialOpts = opts
	}
}

// WithPulseInterval is a ClientOption will set the rate at which each metric
// will be sent to the IngressClient.
func WithPulseInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		c.pulseInterval = d
	}
}

// WithSourceID is a ClientOption that will set the SourceID to be set on
// every envelope sent to the IngressClient.
func WithSourceID(s string) ClientOption {
	return func(c *Client) {
		c.sourceID = s
	}
}

// WithOrigin is a ClientOption that will set an origin tag to be added
// to every envelope sent to the IngressClient.
func WithOrigin(name string) ClientOption {
	return func(c *Client) {
		c.tags["origin"] = name
	}
}

// WithDeployment is a ClientOption that will set a deployment, job and index
// tab on every envelope sent to the IngressClient.
func WithDeployment(deployment, job, index string) ClientOption {
	return func(c *Client) {
		c.tags["deployment"] = deployment
		c.tags["job"] = job
		c.tags["index"] = index
	}
}

// NewClient initializes a new Client and opens a gRPC connection to the given
// address.
func NewClient(addr string, opts ...ClientOption) (*Client, error) {
	client := &Client{
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

// NewCounter will return a new Counter metric that can be incremented. The
// value of the counter will be sent to the Clients IngressClient at the
// interval configured on the Client. When the counters value is sent to the
// IngressClient the value is reset to 0.
func (c *Client) NewCounter(name string, opts ...MetricOption) *Counter {
	opts = append(opts, WithTags(c.tags))
	m := NewCounter(name, c.sourceID, opts...)
	go c.pulse(m)

	return m
}

// NewGauge will return a new Gauge metric that has a value that can be set.
// The value of the gauge will be sent to the Clients IngressClient at the
// interval configured on the Client.
func (c *Client) NewGauge(name, unit string, opts ...MetricOption) *Gauge {
	opts = append(opts, WithTags(c.tags))
	m := NewGauge(name, unit, c.sourceID, opts...)
	go c.pulse(m)

	return m
}

func (c *Client) pulse(s sendable) {
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
