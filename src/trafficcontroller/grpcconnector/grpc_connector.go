package grpcconnector

import (
	"diodes"
	"doppler/dopplerservice"
	"fmt"
	"io"
	"log"
	"plumbing"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
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
	finder         Finder
	mu             sync.Mutex
	clients        []*dopplerClientInfo
	consumerStates []*consumerState
	bufferSize     int
	maxMissed      int
	batcher        MetaMetricBatcher
}

// New creates a new GRPCConnector.
func New(bufferSize, maxMissed int, f Finder, batcher MetaMetricBatcher) *GRPCConnector {
	c := &GRPCConnector{
		bufferSize: bufferSize,
		maxMissed:  maxMissed,
		finder:     f,
		batcher:    batcher,
	}
	go c.readFinder()
	return c
}

// Subscribe returns a Receiver that yields all corresponding messages from Doppler
func (c *GRPCConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) Receiver {
	return c.handleRequest(ctx, req)
}

// ContainerMetrics returns the current container metrics for an app ID.
func (c *GRPCConnector) ContainerMetrics(ctx context.Context, appID string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *GRPCConnector) handleRequest(ctx context.Context, req *plumbing.SubscriptionRequest) Receiver {
	cs := &consumerState{
		ctx:       ctx,
		req:       req,
		maxMissed: c.maxMissed,
		batcher:   c.batcher,
	}
	cs.data = diodes.NewManyToOne(c.bufferSize, cs)

	c.addConsumerState(cs)

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, client := range c.clients {
		go consumeSubscription(cs, client, c.batcher)
	}
	return cs
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

		for _, cs := range c.consumerStates {
			go consumeSubscription(cs, client, c.batcher)
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
	for {
		resp, err := s.Recv()
		if err != nil {
			return err
		}

		batcher.BatchCounter("listeners.receivedEnvelopes").
			SetTag("protocol", "grpc").
			Increment()

		cs.data.Set(resp.Payload)
	}
}

func (c *GRPCConnector) addConsumerState(cs *consumerState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consumerStates = append(c.consumerStates, cs)
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
	data      *diodes.ManyToOne
	missed    int
	maxMissed int
	batcher   MetaMetricBatcher
}

func (cs *consumerState) Recv() ([]byte, error) {
	for {
		if cs.missed > cs.maxMissed {
			return nil, fmt.Errorf("slow of a consumer")
		}

		data, ok := cs.data.TryNext()
		if ok {
			if cs.missed < 0 {
				cs.missed--
			}
			return data, nil
		}

		select {
		case <-cs.ctx.Done():
			return nil, cs.ctx.Err()
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

func (cs *consumerState) Alert(missed int) {
	cs.missed += missed
	log.Printf("Dropped %d messages for %+v request", missed, cs.req)
	cs.batcher.BatchAddCounter("writers.shedEnvelopes", uint64(missed))
}
