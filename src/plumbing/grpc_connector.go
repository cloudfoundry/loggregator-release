package plumbing

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"code.cloudfoundry.org/loggregator/metricemitter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
)

const (
	maxConnections = 2000
)

// DopplerPool creates a pool of doppler gRPC connections
type DopplerPool interface {
	RegisterDoppler(addr string)
	Subscribe(dopplerAddr string, ctx context.Context, req *SubscriptionRequest) (Doppler_BatchSubscribeClient, error)

	Close(dopplerAddr string)
}

type Event struct {
	GRPCDopplers []string
}

// Finder yields events that tell us what dopplers are available.
type Finder interface {
	Next() Event
}

// GRPCConnector establishes GRPC connections to dopplers and allows calls to
// Firehose, Stream, etc to be reduced down to a single Receiver.
type GRPCConnector struct {
	mu      sync.RWMutex
	clients []*dopplerClientInfo

	pool           DopplerPool
	finder         Finder
	consumerStates []unsafe.Pointer
	bufferSize     int

	ingressMetric   *metricemitter.Counter
	recentLogsError *metricemitter.Counter
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

// NewGRPCConnector creates a new GRPCConnector.
func NewGRPCConnector(
	bufferSize int,
	pool DopplerPool,
	f Finder,
	m MetricClient,
) *GRPCConnector {
	ingressMetric := m.NewCounter("ingress",
		metricemitter.WithTags(map[string]string{
			"protocol": "grpc",
		}),
		metricemitter.WithVersion(2, 0),
	)
	recentLogsError := m.NewCounter("query_error",
		metricemitter.WithTags(map[string]string{
			"query": "recent_logs",
		}),
		metricemitter.WithVersion(2, 0),
	)

	c := &GRPCConnector{
		bufferSize:      bufferSize,
		pool:            pool,
		finder:          f,
		consumerStates:  make([]unsafe.Pointer, maxConnections),
		ingressMetric:   ingressMetric,
		recentLogsError: recentLogsError,
	}
	go c.readFinder()
	return c
}

// Subscribe returns a Receiver that yields all corresponding messages from Doppler
func (c *GRPCConnector) Subscribe(ctx context.Context, req *SubscriptionRequest) (recv func() ([]byte, error), err error) {
	cs := &consumerState{
		data:     make(chan []byte, c.bufferSize),
		errs:     make(chan error, 1),
		ctx:      ctx,
		req:      req,
		dopplers: make(map[string]bool),
	}

	go func() {
		<-cs.ctx.Done()
		atomic.StoreInt64(&cs.dead, 1)
	}()

	err = c.addConsumerState(cs)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	log.Printf("Connecting to %d dopplers", len(c.clients))
	for _, client := range c.clients {
		go c.consumeSubscription(cs, client)
	}
	return cs.Recv, nil
}

func (c *GRPCConnector) readFinder() {
	for {
		e := c.finder.Next()
		log.Printf("Event from finder: %+v", e)
		c.handleFinderEvent(e.GRPCDopplers)
	}
}

func (c *GRPCConnector) handleFinderEvent(uris []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newURIs, deadClients := c.delta(uris)

	for _, addr := range newURIs {
		c.pool.RegisterDoppler(addr)
		client := &dopplerClientInfo{
			uri: addr,
		}

		c.clients = append(c.clients, client)

		for i := range c.consumerStates {
			cs := atomic.LoadPointer(&c.consumerStates[i])
			if cs != nil &&
				(*consumerState)(cs) != nil &&
				atomic.LoadInt64(&(*consumerState)(cs).dead) == 0 {
				go c.consumeSubscription((*consumerState)(cs), client)
			}
		}
	}

	for _, deadClient := range deadClients {
		log.Printf("Disabling reconnects for doppler %s", deadClient.uri)
		deadClient.disconnect = true

		if atomic.LoadInt64(&deadClient.refCount) == 0 {
			log.Printf("closing doppler connection %s...", deadClient.uri)
			c.pool.Close(deadClient.uri)
			c.close(deadClient)
		}
	}
}

func (c *GRPCConnector) delta(uris []string) (add []string, dead []*dopplerClientInfo) {
	dead = make([]*dopplerClientInfo, len(c.clients))
	copy(dead, c.clients)

	for _, newURI := range uris {
		var contained bool
		dead, contained = c.subtractClient(dead, newURI)
		if !contained {
			add = append(add, newURI)
		}
	}
	return add, dead
}

func (c *GRPCConnector) subtractClient(remaining []*dopplerClientInfo, uri string) ([]*dopplerClientInfo, bool) {
	for i, client := range remaining {
		if uri == client.uri {
			client.disconnect = false
			return append(remaining[:i], remaining[i+1:]...), true
		}
	}
	return remaining, false
}

func (c *GRPCConnector) close(client *dopplerClientInfo) {
	for i, clt := range c.clients {
		if clt == client {
			c.clients = append(c.clients[:i], c.clients[i+1:]...)
		}
	}
}

func (c *GRPCConnector) consumeSubscription(cs *consumerState, dopplerClient *dopplerClientInfo) {
	if !cs.tryAddDoppler(dopplerClient.uri) {
		return
	}

	atomic.AddInt64(&dopplerClient.refCount, 1)
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if atomic.AddInt64(&dopplerClient.refCount, -1) <= 0 &&
			dopplerClient.disconnect {
			log.Printf("closing doppler connection %s...", dopplerClient.uri)
			c.pool.Close(dopplerClient.uri)
			c.close(dopplerClient)
		}

		cs.forgetDoppler(dopplerClient.uri)
	}()

	delay := time.Millisecond

	tried := false
	for {
		ctxDisconnect := atomic.LoadInt64(&cs.dead)
		c.mu.RLock()
		dopplerDisconnect := dopplerClient.disconnect
		c.mu.RUnlock()
		if tried && dopplerDisconnect || ctxDisconnect != 0 {
			log.Printf("Disconnecting from stream (%s) (doppler.disconnect=%v) (ctx.disconnect=%d)", dopplerClient.uri, dopplerDisconnect, ctxDisconnect)
			return
		}
		tried = true

		dopplerStream, err := c.pool.Subscribe(dopplerClient.uri, cs.ctx, cs.req)

		if err != nil {
			log.Printf("Unable to connect to doppler (%s): %s", dopplerClient.uri, err)
			time.Sleep(delay)
			if delay < time.Minute {
				delay *= 10
			}
			continue
		}

		delay = time.Millisecond

		if err := c.readStream(dopplerStream, cs); err != nil {
			status, ok := status.FromError(err)
			if ok && status.Code() != codes.Canceled {
				log.Printf("error getting logs from provider: %s", err)
			}
			continue
		}
	}
}

func (c *GRPCConnector) readStream(s Doppler_BatchSubscribeClient, cs *consumerState) error {
	for {
		// Recv() will return an error if the context has been cancelled.
		resp, err := s.Recv()
		if err != nil {
			return err
		}

		for _, p := range resp.Payload {
			select {
			case <-cs.ctx.Done():
				return nil
			case cs.data <- p:
			}
		}

		// metric-documentation-v2: (ingress) Number of v1 envelopes received over
		// gRPC from Dopplers.
		c.ingressMetric.Increment(uint64(len(resp.Payload)))
	}
}

func (c *GRPCConnector) addConsumerState(cs *consumerState) error {
	for i := range c.consumerStates {
		state := atomic.LoadPointer(&c.consumerStates[i])
		if state == nil ||
			(*consumerState)(state) == nil ||
			atomic.LoadInt64(&(*consumerState)(state).dead) != 0 {

			if atomic.CompareAndSwapPointer(&c.consumerStates[i], state, unsafe.Pointer(cs)) {
				return nil
			}
		}
	}

	return fmt.Errorf("at connection limit: %d", maxConnections)
}

type dopplerClientInfo struct {
	uri        string
	disconnect bool
	refCount   int64
}

type consumerState struct {
	ctx       context.Context
	req       *SubscriptionRequest
	data      chan []byte
	errs      chan error
	missed    int
	maxMissed int
	dead      int64

	mu       sync.Mutex
	dopplers map[string]bool
}

func (cs *consumerState) Recv() ([]byte, error) {
	select {
	case err := <-cs.errs:
		return nil, err
	case data := <-cs.data:
		return data, nil
	case <-cs.ctx.Done():
		return nil, cs.ctx.Err()
	}
}

func (cs *consumerState) tryAddDoppler(doppler string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	ok, _ := cs.dopplers[doppler]
	if ok {
		return false
	}

	cs.dopplers[doppler] = true
	return true
}

func (cs *consumerState) forgetDoppler(doppler string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.dopplers, doppler)
}
