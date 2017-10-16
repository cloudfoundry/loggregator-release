package plumbing

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	throughputlb "code.cloudfoundry.org/grpc-throughputlb"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Pool struct {
	mu                 sync.RWMutex
	dopplers           map[string]clientInfo
	dialOpts           []grpc.DialOption
	maxRequestsPerConn int
}

type clientInfo struct {
	client DopplerClient
	closer io.Closer
}

func NewPool(maxRequestsPerConn int, opts ...grpc.DialOption) *Pool {
	return &Pool{
		dopplers:           make(map[string]clientInfo),
		dialOpts:           opts,
		maxRequestsPerConn: maxRequestsPerConn,
	}
}

func (p *Pool) RegisterDoppler(addr string) {
	go p.connectToDoppler(addr)
}

func (p *Pool) Subscribe(dopplerAddr string, ctx context.Context, req *SubscriptionRequest) (Doppler_BatchSubscribeClient, error) {
	p.mu.RLock()
	ci, ok := p.dopplers[dopplerAddr]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no connections available for subscription")
	}

	return ci.client.BatchSubscribe(ctx, req, grpc.FailFast(false))
}

func (p *Pool) ContainerMetrics(dopplerAddr string, ctx context.Context, req *ContainerMetricsRequest) (*ContainerMetricsResponse, error) {
	p.mu.RLock()
	ci, ok := p.dopplers[dopplerAddr]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no connections available for container metrics")
	}

	return ci.client.ContainerMetrics(ctx, req, grpc.FailFast(false))
}

func (p *Pool) RecentLogs(dopplerAddr string, ctx context.Context, req *RecentLogsRequest) (*RecentLogsResponse, error) {
	p.mu.RLock()
	ci, ok := p.dopplers[dopplerAddr]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no connections available for recent logs")
	}

	return ci.client.RecentLogs(ctx, req, grpc.FailFast(false))
}

func (p *Pool) Close(dopplerAddr string) {
	p.mu.Lock()
	ci, ok := p.dopplers[dopplerAddr]
	delete(p.dopplers, dopplerAddr)
	p.mu.Unlock()

	if ok {
		ci.closer.Close()
	}
}

func (p *Pool) connectToDoppler(addr string) {
	opts := []grpc.DialOption{
		grpc.WithBalancer(
			throughputlb.NewThroughputLoadBalancer(100, p.maxRequestsPerConn),
		),
	}

	for _, o := range p.dialOpts {
		opts = append(opts, o)
	}

	for {
		// TODO: Do we need this log line? It prints the same log line based
		// on pool size
		log.Printf("adding doppler %s", addr)

		conn, err := grpc.Dial(addr, opts...)
		if err != nil {
			// TODO: We don't yet understand how this could happen, we should.
			// TODO: Replace with exponential backoff.
			log.Printf("unable to subscribe to doppler %s: %s", addr, err)
			time.Sleep(5 * time.Second)
			continue
		}

		p.mu.Lock()
		p.dopplers[addr] = clientInfo{
			client: NewDopplerClient(conn),
			closer: conn,
		}
		p.mu.Unlock()

		return
	}
}
