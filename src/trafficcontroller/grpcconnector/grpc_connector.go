package grpcconnector

import (
	"doppler/dopplerservice"
	"errors"
	"fmt"
	"io"
	"log"
	"plumbing"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	maxConnections = 2000
)

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

	finder         Finder
	consumerStates []unsafe.Pointer
	bufferSize     int
	maxMissed      int
	batcher        MetaMetricBatcher
}

// New creates a new GRPCConnector.
func New(bufferSize, maxMissed int, f Finder, batcher MetaMetricBatcher) *GRPCConnector {
	c := &GRPCConnector{
		bufferSize:     bufferSize,
		maxMissed:      maxMissed,
		finder:         f,
		batcher:        batcher,
		consumerStates: make([]unsafe.Pointer, maxConnections),
	}
	go c.readFinder()
	return c
}

// Subscribe returns a Receiver that yields all corresponding messages from Doppler
func (c *GRPCConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (Receiver, error) {
	return c.handleRequest(ctx, req)
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
		nextResp, err := client.client.ContainerMetrics(ctx, req)
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
		nextResp, err := client.client.RecentLogs(ctx, req)
		if err != nil {
			log.Printf("error from doppler (%s) while fetching recent logs: %s", client.uri, err)
			continue
		}

		resp = append(resp, nextResp.Payload...)
	}
	return resp
}

func (c *GRPCConnector) handleRequest(ctx context.Context, req *plumbing.SubscriptionRequest) (Receiver, error) {
	cs := &consumerState{
		data:      make(chan []byte),
		errs:      make(chan error, 1),
		ctx:       ctx,
		req:       req,
		maxMissed: c.maxMissed,
		batcher:   c.batcher,
	}

	err := c.addConsumerState(cs)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, client := range c.clients {
		go consumeSubscription(cs, client, c.batcher)
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
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			// TODO: We don't yet understand how this could happen, we should.
			// TODO: Replace with exponential backoff.
			log.Printf("Unable to Dial doppler %s: %s", addr, err)
			time.Sleep(5 * time.Second)
			continue
		}

		client := &dopplerClientInfo{
			client: plumbing.NewDopplerClient(conn),
			uri:    addr,
			closer: conn,
		}
		c.clients = append(c.clients, client)

		for i := range c.consumerStates {
			cs := atomic.LoadPointer(&c.consumerStates[i])
			if cs != nil &&
				(*consumerState)(cs) != nil &&
				atomic.LoadInt64(&(*consumerState)(cs).dead) == 0 {
				go consumeSubscription((*consumerState)(cs), client, c.batcher)
			}
		}
	}

	for _, deadClient := range deadClients {
		log.Printf("Disabling reconnects for doppler %s", deadClient.uri)
		for i, clt := range c.clients {
			if clt == deadClient {
				c.clients = append(c.clients[:i], c.clients[i+1:]...)
			}
		}
		atomic.StoreInt64(&deadClient.disconnect, 1)

		if atomic.LoadInt64(&deadClient.refCount) == 0 {
			deadClient.close()
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
			return append(remaining[:i], remaining[i+1:]...), true
		}
	}
	return remaining, false
}

func consumeSubscription(cs *consumerState, dopplerClient *dopplerClientInfo, batcher MetaMetricBatcher) {
	atomic.AddInt64(&dopplerClient.refCount, 1)
	defer func() {
		if atomic.AddInt64(&dopplerClient.refCount, -1) <= 0 &&
			atomic.LoadInt64(&dopplerClient.disconnect) != 0 {
			dopplerClient.close()
		}
	}()

	var ctxDone int64
	go func() {
		<-cs.ctx.Done()
		atomic.StoreInt64(&ctxDone, 1)
		atomic.StoreInt64(&cs.dead, 1)
	}()

	for {
		dopplerDisconnect := atomic.LoadInt64(&dopplerClient.disconnect)
		ctxDisconnect := atomic.LoadInt64(&ctxDone)
		if dopplerDisconnect != 0 || ctxDisconnect != 0 {
			log.Printf("Disconnecting from stream (%s) (doppler.disconnect=%d) (ctx.disconnect=%d)", dopplerClient.uri, dopplerDisconnect, ctxDisconnect)
			return
		}

		dopplerStream, err := dopplerClient.client.Subscribe(cs.ctx, cs.req)

		if err != nil {
			log.Printf("Unable to connect to doppler (%s): %s", dopplerClient.uri, err)
			// TODO: Replace with exponential backoff.
			time.Sleep(2 * time.Second)
			continue
		}

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
	client     plumbing.DopplerClient
	closer     io.Closer
	uri        string
	disconnect int64
	refCount   int64
}

func (c dopplerClientInfo) close() {
	log.Printf("Closing doppler (%s) connection...", c.uri)
	if err := c.closer.Close(); err != nil {
		log.Printf("Error closing doppler (%s): %s", c.uri, err)
	}
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

func (cs *consumerState) Alert(missed int) {
	cs.missed += missed
	log.Printf("Dropped %d messages for %+v request", missed, cs.req)
	cs.batcher.BatchAddCounter("writers.shedEnvelopes", uint64(missed))
}
