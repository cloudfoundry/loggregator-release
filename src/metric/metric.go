package metric

import (
	"context"
	"diodes"
	"fmt"
	"log"
	"sync"
	"time"

	v2 "plumbing/v2"

	"google.golang.org/grpc"
)

var (
	mu sync.Mutex

	client      v2.MetronIngressClient
	sender      v2.MetronIngress_SenderClient
	conf        *config
	batchBuffer *diodes.ManyToOneEnvelopeV2
)

type config struct {
	consumerAddr  string
	dialOpts      []grpc.DialOption
	sourceUUID    string
	batchInterval time.Duration
	prefix        string
	component     string
}

type SetOpts func(c *config)

func WithGrpcDialOpts(opts ...grpc.DialOption) func(c *config) {
	return func(c *config) {
		c.dialOpts = opts
	}
}

func WithSourceUUID(id string) func(c *config) {
	return func(c *config) {
		c.sourceUUID = id
	}
}

func WithAddr(addr string) func(c *config) {
	return func(c *config) {
		c.consumerAddr = addr
	}
}

func WithBatchInterval(interval time.Duration) func(c *config) {
	return func(c *config) {
		c.batchInterval = interval
	}
}

func WithPrefix(prefix string) func(c *config) {
	return func(c *config) {
		c.prefix = prefix
	}
}

func WithComponent(name string) func(c *config) {
	return func(c *config) {
		c.component = name
	}
}

func Setup(opts ...SetOpts) {
	mu.Lock()
	defer mu.Unlock()

	conf = &config{
		consumerAddr:  "localhost:3458",
		dialOpts:      []grpc.DialOption{grpc.WithInsecure()},
		batchInterval: 10 * time.Second,
		prefix:        "loggregator",
	}

	for _, opt := range opts {
		opt(conf)
	}

	conn, err := grpc.Dial(conf.consumerAddr, conf.dialOpts...)
	if err != nil {
		log.Printf("Failed to connect to metric consumer: %s", err)
		return
	}

	client = v2.NewMetronIngressClient(conn)
	sender, err = client.Sender(context.Background())

	if err != nil {
		log.Printf("Failed to get sender from metric consumer: %s", err)
		return
	}

	batchBuffer = diodes.NewManyToOneEnvelopeV2(1000, diodes.AlertFunc(func(missed int) {
		log.Printf("dropped metrics %d", missed)
	}))

	go runBatcher()
}

type IncrementOpt func(*incrementOption)

type incrementOption struct {
	delta uint64
}

func WithIncrement(delta uint64) func(*incrementOption) {
	return func(i *incrementOption) {
		i.delta = delta
	}
}

func IncCounter(name string, options ...IncrementOpt) {
	incConf := &incrementOption{
		delta: 1,
	}

	for _, opt := range options {
		opt(incConf)
	}

	e := &v2.Envelope{
		SourceUuid: conf.sourceUUID,
		Timestamp:  time.Now().UnixNano(),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: fmt.Sprintf("%s.%s", conf.prefix, name),
				Value: &v2.Counter_Delta{
					Delta: incConf.delta,
				},
			},
		},
		Tags: map[string]*v2.Value{
			"component": &v2.Value{
				Data: &v2.Value_Text{
					Text: conf.component,
				},
			},
		},
	}

	batchBuffer.Set(e)
}

func runBatcher() {
	ticker := time.NewTicker(conf.batchInterval)
	defer ticker.Stop()

	for range ticker.C {
		for _, e := range aggregateCounters() {
			sender.Send(e)
		}
	}
}

func aggregateCounters() map[string]*v2.Envelope {
	m := make(map[string]*v2.Envelope)
	for {
		envelope, ok := batchBuffer.TryNext()
		if !ok {
			break
		}

		existingEnvelope, ok := m[envelope.GetCounter().Name]
		if !ok {
			existingEnvelope = envelope
			m[envelope.GetCounter().Name] = existingEnvelope
			continue
		}

		existingEnvelope.GetCounter().GetValue().(*v2.Counter_Delta).Delta += envelope.GetCounter().GetDelta()
	}

	return m
}
