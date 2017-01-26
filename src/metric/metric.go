package metric

import (
	"context"
	"diodes"
	"log"
	"sync"
	"time"

	v2 "plumbing/v2"

	"google.golang.org/grpc"
)

var (
	mu sync.Mutex

	client v2.MetronIngressClient
	sender v2.MetronIngress_SenderClient
	conf   *config
)

type config struct {
	consumerAddr  string
	dialOpts      []grpc.DialOption
	sourceUUID    string
	batchInterval time.Duration
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

func Setup(opts ...SetOpts) {
	mu.Lock()
	defer mu.Unlock()

	conf = &config{
		consumerAddr:  "localhost:3458",
		dialOpts:      []grpc.DialOption{grpc.WithInsecure()},
		batchInterval: 10 * time.Second,
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

	go runBatcher()
}

func IncCounter(name string) {
	e := &v2.Envelope{
		SourceUuid: conf.sourceUUID,
		Timestamp:  time.Now().UnixNano(),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: name,
				Value: &v2.Counter_Delta{
					Delta: 1,
				},
			},
		},
	}

	if err := sender.Send(e); err != nil {
		log.Printf("Failed to send envelope: %s", err)
	}
}

func runBatcher(buffer diodes.ManyToOneEnvelopeV2) {
	ticker := time.NewTicker(conf.batchInterval)
	defer ticker.Stop()

	for range ticker.C {

	}
}
