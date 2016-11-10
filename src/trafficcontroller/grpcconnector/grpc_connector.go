package grpcconnector

import (
	"doppler/dopplerservice"
	"errors"
	"fmt"
	"log"
	"plumbing"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"golang.org/x/net/context"
)

const (
	maxConnections = 2000
)

// DopplerPool creates a pool of doppler gRPC connections
type DopplerPool interface {
	RegisterDoppler(addr string)
	Subscribe(dopplerAddr string, ctx context.Context, req *plumbing.SubscriptionRequest) (plumbing.Doppler_SubscribeClient, error)
	ContainerMetrics(dopplerAddr string, ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error)
	RecentLogs(dopplerAddr string, ctx context.Context, req *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error)

	Close(dopplerAddr string)
}

// Finder yields events that tell us what dopplers are available.
type Finder interface {
	Next() dopplerservice.Event
}

type MetaMetricBatcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
	BatchAddCounter(name string, delta uint64)
}

// Receiver yeilds messages from a pool of Dopplers.
type Receiver interface {
	Recv() ([]byte, error)
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
	batcher        MetaMetricBatcher
}

// New creates a new GRPCConnector.
func New(bufferSize int, pool DopplerPool, f Finder, batcher MetaMetricBatcher) *GRPCConnector {
	c := &GRPCConnector{
		bufferSize:     bufferSize,
		pool:           pool,
		finder:         f,
		batcher:        batcher,
		consumerStates: make([]unsafe.Pointer, maxConnections),
	}
	go c.readFinder()
	return c
}

// ContainerMetrics returns the current container metrics for an app ID.
func (c *GRPCConnector) ContainerMetrics(ctx context.Context, appID string) [][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var resp [][]byte
	for _, client := range c.clients {
		req := &plumbing.ContainerMetricsRequest{
			AppID: appID,
		}
		nextResp, err := c.pool.ContainerMetrics(client.uri, ctx, req)
		if err != nil {
			log.Printf("error from doppler (%s) while fetching container metrics: %s", client.uri, err)
			continue
		}

		resp = append(resp, nextResp.Payload...)
	}
	return resp
}

// RecentLogs returns the current recent logs for an app ID.
func (c *GRPCConnector) RecentLogs(ctx context.Context, appID string) [][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var resp [][]byte
	for _, client := range c.clients {
		req := &plumbing.RecentLogsRequest{
			AppID: appID,
		}
		nextResp, err := c.pool.RecentLogs(client.uri, ctx, req)
		if err != nil {
			log.Printf("error from doppler (%s) while fetching recent logs: %s", client.uri, err)
			continue
		}

		resp = append(resp, nextResp.Payload...)
	}
	return resp
}

// Subscribe returns a Receiver that yields all corresponding messages from Doppler
func (c *GRPCConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (Receiver, error) {
	cs := &consumerState{
		data:     make(chan []byte, c.bufferSize),
		errs:     make(chan error, 1),
		ctx:      ctx,
		req:      req,
		batcher:  c.batcher,
		dopplers: make(map[string]bool),
	}

	go func() {
		<-cs.ctx.Done()
		atomic.StoreInt64(&cs.dead, 1)
	}()

	err := c.addConsumerState(cs)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	log.Printf("Connecting to %d dopplers", len(c.clients))
	for _, client := range c.clients {
		go c.consumeSubscription(cs, client, c.batcher)
	}
	return cs, nil
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
		log.Printf("Adding doppler %s", addr)
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
				go c.consumeSubscription((*consumerState)(cs), client, c.batcher)
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

func (c *GRPCConnector) consumeSubscription(cs *consumerState, dopplerClient *dopplerClientInfo, batcher MetaMetricBatcher) {
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

		if err := readStream(dopplerStream, cs, batcher); err != nil {
			log.Printf("Error while reading from stream (%s): %s", dopplerClient.uri, err)
			continue
		}
	}
}

type plumbingReceiver interface {
	Recv() (*plumbing.Response, error)
}

func readStream(s plumbingReceiver, cs *consumerState, batcher MetaMetricBatcher) error {
	timer := time.NewTimer(time.Second)
	timer.Stop()
	for {
		resp, err := s.Recv()
		if err != nil {
			return err
		}

		batcher.BatchCounter("listeners.receivedEnvelopes").
			SetTag("protocol", "grpc").
			Increment()

		timer.Reset(time.Second)
		select {
		case cs.data <- resp.Payload:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			cs.batcher.BatchAddCounter("grpcConnector.slowConsumers", 1)
			writeError(errors.New("GRPCConnector: slow consumer"), cs.errs)
		}
	}
}

func writeError(err error, c chan<- error) {
	select {
	case c <- err:
	default:
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
	req       *plumbing.SubscriptionRequest
	data      chan []byte
	errs      chan error
	missed    int
	maxMissed int
	batcher   MetaMetricBatcher
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
