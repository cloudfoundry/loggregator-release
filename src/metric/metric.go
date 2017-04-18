package metric

import (
	"context"
	"diodes"
	"fmt"
	"log"
	"sync"
	"time"

	gendiodes "github.com/cloudfoundry/diodes"

	v2 "plumbing/v2"

	"google.golang.org/grpc"
)

type Emitter struct {
	consumerAddr  string
	dialOpts      []grpc.DialOption
	sourceID      string
	batchInterval time.Duration
	tags          map[string]string

	client      v2.IngressClient
	batchBuffer *diodes.ManyToOneEnvelopeV2

	mu sync.RWMutex
	s  v2.Ingress_SenderClient
}

type EmitterOpt func(*Emitter)

func WithGrpcDialOpts(opts ...grpc.DialOption) func(e *Emitter) {
	return func(e *Emitter) {
		e.dialOpts = opts
	}
}

func WithSourceID(id string) func(e *Emitter) {
	return func(e *Emitter) {
		e.sourceID = id
	}
}

func WithAddr(addr string) func(e *Emitter) {
	return func(e *Emitter) {
		e.consumerAddr = addr
	}
}

func WithBatchInterval(interval time.Duration) func(e *Emitter) {
	return func(e *Emitter) {
		e.batchInterval = interval
	}
}

func WithOrigin(name string) func(e *Emitter) {
	return func(e *Emitter) {
		e.tags["origin"] = name
	}
}

func WithDeploymentMeta(deployment, job, index string) func(e *Emitter) {
	return func(e *Emitter) {
		e.tags["deployment"] = deployment
		e.tags["job"] = job
		e.tags["index"] = index
	}
}

func New(opts ...EmitterOpt) (*Emitter, error) {
	e := &Emitter{
		consumerAddr:  "localhost:3458",
		dialOpts:      []grpc.DialOption{grpc.WithInsecure()},
		batchInterval: 10 * time.Second,
		tags:          map[string]string{"prefix": "loggregator"},
	}

	for _, opt := range opts {
		opt(e)
	}
	e.dialOpts = append(e.dialOpts, grpc.WithBackoffMaxDelay(time.Second))

	e.batchBuffer = diodes.NewManyToOneEnvelopeV2(1000, gendiodes.AlertFunc(func(missed int) {
		log.Printf("dropped metrics %d", missed)
	}))

	conn, err := grpc.Dial(e.consumerAddr, e.dialOpts...)
	if err != nil {
		return nil, err
	}

	e.client = v2.NewIngressClient(conn)
	e.s, err = e.client.Sender(context.Background())
	if err != nil {
		log.Printf("Failed to get sender from metric consumer: %s", err)
	}

	go e.runBatcher()
	go e.maintainer()

	return e, nil
}

func (e *Emitter) runBatcher() {
	ticker := time.NewTicker(e.batchInterval)
	defer ticker.Stop()

	for range ticker.C {
		s := e.sender()

		if s == nil {
			continue
		}

		for _, envelope := range e.aggregateCounters() {
			s.Send(envelope)
		}
	}
}

func (e *Emitter) aggregateCounters() map[string]*v2.Envelope {
	m := make(map[string]*v2.Envelope)
	for {
		envelope, ok := e.batchBuffer.TryNext()
		if !ok {
			break
		}

		// BUG: we need to batch by tags as well as name
		existingEnvelope, ok := m[envelope.GetCounter().Name]
		if !ok {
			m[envelope.GetCounter().Name] = envelope
			continue
		}

		value := existingEnvelope.GetCounter().GetValue().(*v2.Counter_Delta)
		value.Delta += envelope.GetCounter().GetDelta()
	}
	return m
}

func (e *Emitter) maintainer() {
	for range time.Tick(time.Second) {
		s := e.sender()

		if s != nil {
			continue
		}

		s, err := e.client.Sender(context.Background())
		if err != nil {
			log.Printf("Failed to get sender from metric consumer: %s (retrying)", err)
			continue
		}

		e.setSender(s)
	}
}

func (e *Emitter) sender() v2.Ingress_SenderClient {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.s
}

func (e *Emitter) setSender(s v2.Ingress_SenderClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.s = s
}

type IncrementOpt func(*incrementOption)

type incrementOption struct {
	delta uint64
	tags  map[string]string
}

func WithIncrement(delta uint64) func(*incrementOption) {
	return func(i *incrementOption) {
		i.delta = delta
	}
}

func WithVersion(major, minor uint) func(*incrementOption) {
	return func(i *incrementOption) {
		i.tags["metric_version"] = fmt.Sprintf("%d.%d", major, minor)
	}
}

func WithTag(name, value string) func(*incrementOption) {
	return func(i *incrementOption) {
		i.tags[name] = value
	}
}

func (e *Emitter) IncCounter(name string, options ...IncrementOpt) {
	if e.batchBuffer == nil {
		return
	}

	incConf := &incrementOption{
		delta: 1,
		tags:  make(map[string]string),
	}

	for _, opt := range options {
		opt(incConf)
	}

	tags := v2Tags(incConf.tags)
	etags := v2Tags(e.tags)
	for k, v := range etags {
		tags[k] = v
	}

	envelope := &v2.Envelope{
		SourceId:  e.sourceID,
		Timestamp: time.Now().UnixNano(),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name: name,
				Value: &v2.Counter_Delta{
					Delta: incConf.delta,
				},
			},
		},
		Tags: tags,
	}

	e.batchBuffer.Set(envelope)
}

type PulseOpt func(*pulse)

func WithPulseTag(name, value string) func(*pulse) {
	return func(p *pulse) {
		p.tags[name] = value
	}
}

func WithPulseInterval(t time.Duration) func(*pulse) {
	return func(p *pulse) {
		p.interval = t
	}
}

func (e *Emitter) PulseCounter(name string, opts ...PulseOpt) func(delta uint64) {
	p := &pulse{
		emitter:  e,
		interval: time.Minute,

		name: name,
		tags: make(map[string]string),
	}
	for _, o := range opts {
		o(p)
	}
	go p.start()

	var incopts []IncrementOpt
	for k, v := range p.tags {
		incopts = append(incopts, WithTag(k, v))
	}
	return func(d uint64) {
		opts := make([]IncrementOpt, len(incopts)+1)
		copy(opts, incopts)
		opts[len(opts)-1] = WithIncrement(d)
		e.IncCounter(
			name,
			opts...,
		)
	}
}

type pulse struct {
	emitter  *Emitter
	interval time.Duration

	name string
	tags map[string]string
}

func (p *pulse) envelope() *v2.Envelope {
	return &v2.Envelope{
		Tags: v2Tags(p.tags),
		Message: &v2.Envelope_Counter{
			Counter: &v2.Counter{
				Name:  p.name,
				Value: &v2.Counter_Delta{},
			},
		},
	}
}

func (p *pulse) start() {
	envelope := p.envelope()
	for {
		time.Sleep(p.interval)

		s := p.emitter.sender()

		if s == nil {
			continue
		}
		s.Send(envelope)
	}
}

func v2Tags(tags map[string]string) map[string]*v2.Value {
	v2tags := make(map[string]*v2.Value)
	for k, v := range tags {
		v2tags[k] = &v2.Value{
			Data: &v2.Value_Text{
				Text: v,
			},
		}
	}
	return v2tags
}
