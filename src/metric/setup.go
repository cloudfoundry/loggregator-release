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

	client      v2.IngressClient
	sender      v2.Ingress_SenderClient
	conf        *config
	batchBuffer *diodes.ManyToOneEnvelopeV2
)

type config struct {
	consumerAddr  string
	dialOpts      []grpc.DialOption
	sourceUUID    string
	batchInterval time.Duration
	tags          map[string]string
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
		c.tags["prefix"] = prefix
	}
}

func WithOrigin(name string) func(c *config) {
	return func(c *config) {
		c.tags["origin"] = name
	}
}

func WithDeploymentMeta(deployment, job, index string) func(c *config) {
	return func(c *config) {
		c.tags["deployment"] = deployment
		c.tags["job"] = job
		c.tags["index"] = index
	}
}

func Setup(opts ...SetOpts) {
	mu.Lock()
	defer mu.Unlock()

	conf = &config{
		consumerAddr:  "localhost:3458",
		dialOpts:      []grpc.DialOption{grpc.WithInsecure()},
		batchInterval: 10 * time.Second,
		tags:          map[string]string{"prefix": "loggregator"},
	}

	for _, opt := range opts {
		opt(conf)
	}

	batchBuffer = diodes.NewManyToOneEnvelopeV2(1000, diodes.AlertFunc(func(missed int) {
		log.Printf("dropped metrics %d", missed)
	}))

	conn, err := grpc.Dial(conf.consumerAddr, conf.dialOpts...)
	if err != nil {
		log.Printf("Failed to connect to metric consumer: %s", err)
		return
	}

	client = v2.NewIngressClient(conn)
	sender, err = client.Sender(context.Background())
	if err != nil {
		log.Printf("Failed to get sender from metric consumer: %s", err)
	}

	go runBatcher()
	go maintainer()
}

func maintainer() {
	for range time.Tick(time.Second) {
		mu.Lock()
		s := sender
		mu.Unlock()

		if s != nil {
			continue
		}

		s, err := client.Sender(context.Background())
		if err != nil {
			log.Printf("Failed to get sender from metric consumer: %s (retrying)", err)
			continue
		}

		mu.Lock()
		sender = s
		mu.Unlock()
	}
}
