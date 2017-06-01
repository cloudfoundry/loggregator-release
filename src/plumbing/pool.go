package plumbing

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Pool struct {
	size int

	mu       sync.RWMutex
	dopplers map[string][]unsafe.Pointer
	dialOpts []grpc.DialOption
}

type clientInfo struct {
	client DopplerClient
	closer io.Closer
}

func NewPool(size int, opts ...grpc.DialOption) *Pool {
	return &Pool{
		size:     size,
		dopplers: make(map[string][]unsafe.Pointer),
		dialOpts: opts,
	}
}

func (p *Pool) RegisterDoppler(addr string) {
	clients := make([]unsafe.Pointer, p.size)

	p.mu.Lock()
	p.dopplers[addr] = clients
	p.mu.Unlock()

	for i := 0; i < p.size; i++ {
		go p.connectToDoppler(addr, clients, i)
	}
}

func (p *Pool) Subscribe(dopplerAddr string, ctx context.Context, req *SubscriptionRequest) (Doppler_SubscribeClient, error) {
	p.mu.RLock()
	clients := p.dopplers[dopplerAddr]
	p.mu.RUnlock()

	client := p.fetchClient(clients)

	if client == nil {
		return nil, fmt.Errorf("no connections available for subscription")
	}

	return client.Subscribe(ctx, req)
}

func (p *Pool) ContainerMetrics(dopplerAddr string, ctx context.Context, req *ContainerMetricsRequest) (*ContainerMetricsResponse, error) {
	p.mu.RLock()
	clients := p.dopplers[dopplerAddr]
	p.mu.RUnlock()

	client := p.fetchClient(clients)

	if client == nil {
		return nil, fmt.Errorf("no connections available for container metrics")
	}

	return client.ContainerMetrics(ctx, req)
}

func (p *Pool) RecentLogs(dopplerAddr string, ctx context.Context, req *RecentLogsRequest) (*RecentLogsResponse, error) {
	p.mu.RLock()
	clients := p.dopplers[dopplerAddr]
	p.mu.RUnlock()

	client := p.fetchClient(clients)

	if client == nil {
		return nil, fmt.Errorf("no connections available for recent logs")
	}

	return client.RecentLogs(ctx, req)
}

func (p *Pool) Close(dopplerAddr string) {
	p.mu.Lock()
	clients := p.dopplers[dopplerAddr]
	delete(p.dopplers, dopplerAddr)
	p.mu.Unlock()

	for i := range clients {
		clt := atomic.LoadPointer(&clients[i])
		if clt == nil ||
			(*clientInfo)(clt) == nil {
			continue
		}

		client := *(*clientInfo)(clt)
		client.closer.Close()
	}
}

func (p *Pool) fetchClient(clients []unsafe.Pointer) DopplerClient {
	seed := rand.Int()
	for i := range clients {
		idx := (i + seed) % p.size
		clt := atomic.LoadPointer(&clients[idx])
		if clt == nil ||
			(*clientInfo)(clt) == nil {
			continue
		}

		client := *(*clientInfo)(clt)
		return client.client
	}

	return nil
}

func (p *Pool) connectToDoppler(addr string, clients []unsafe.Pointer, idx int) {
	for {
		// TODO: Do we need this log line? It prints the same log line based
		// on pool size
		log.Printf("adding doppler %s", addr)

		conn, err := grpc.Dial(addr, p.dialOpts...)
		if err != nil {
			// TODO: We don't yet understand how this could happen, we should.
			// TODO: Replace with exponential backoff.
			log.Printf("unable to subscribe to doppler %s: %s", addr, err)
			time.Sleep(5 * time.Second)
			continue
		}

		client := NewDopplerClient(conn)
		info := clientInfo{
			client: client,
			closer: conn,
		}

		atomic.StorePointer(&clients[idx], unsafe.Pointer(&info))
		return
	}
}
