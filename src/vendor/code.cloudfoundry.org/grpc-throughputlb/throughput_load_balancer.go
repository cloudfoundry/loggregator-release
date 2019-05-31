package throughputlb

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	errUnavailable         = grpc.Errorf(codes.Unavailable, "there is no address available")
	errMaxRequestsExceeded = errors.New("max requests exceeded")
)

type addrState int64

const (
	stateDown addrState = iota
	stateUp
)

// ThroughputLoadBalancer is a gRPC load balancer that satisfies the
// grpc.Balancer interface. This load balancer will open multiple connections
// to a single address and enfoce a maximum number of concurrent requests per
// connection.
type ThroughputLoadBalancer struct {
	mu    sync.RWMutex
	addrs []*address

	target      string
	notify      chan []grpc.Address
	maxRequests int
	numAddrs    int
}

// NewThroughputLoadBalancer returns an initialized throughput load balancer
// with the given number of maxRequests per address and max number of
// addresses. This throughput load balancer can be passed to gRPC as a dial
// option.
//
//	lb := throughputlb.NewThroughputLoadBalancer(100, 20)
//	conn, err := grpc.Dial("localhost:8080", grpc.WithBalancer(lb))
//	...
func NewThroughputLoadBalancer(
	maxRequests int,
	numAddrs int,
) *ThroughputLoadBalancer {
	lb := &ThroughputLoadBalancer{
		notify:      make(chan []grpc.Address, numAddrs),
		addrs:       make([]*address, numAddrs),
		maxRequests: maxRequests,
		numAddrs:    numAddrs,
	}

	return lb
}

// Start is called by gRPC when dial is called to configure the load balancer
// with a target and a BalancerConfig. The BalancerConfig is not used by this
// load balancer. When start is called by gRPC the load balancer will be
// configured with a configured number of addresses and notify gRPC of those
// addresses. All of the address targets are the same with an integer as
// metidata to make them unique.
func (lb *ThroughputLoadBalancer) Start(target string, cfg grpc.BalancerConfig) error {
	lb.mu.Lock()
	lb.target = target
	for i := 0; i < lb.numAddrs; i++ {
		lb.addrs[i] = &address{
			Address: grpc.Address{
				Addr:     lb.target,
				Metadata: i,
			},
			maxRequests: lb.maxRequests,
		}
	}
	lb.sendNotify()
	lb.mu.Unlock()

	return nil
}

// Up is called by gRPC when a connection has been established for the given
// address. Internally in the load balancer we will mark the address as up and
// it can now be used by calling Get. The function returned by this method
// will be called by gRPC when the address goes down and the address is no
// longer usable.
func (lb *ThroughputLoadBalancer) Up(addr grpc.Address) func(error) {
	lb.mu.RLock()
	addrs := lb.addrs
	lb.mu.RUnlock()

	for _, a := range addrs {
		if a.Address == addr {
			lb.mu.Lock()
			a.goUp()
			lb.mu.Unlock()

			return func(err error) {
				lb.mu.Lock()
				a.goDown(err)
				lb.mu.Unlock()
			}
		}
	}

	return func(_ error) {}
}

// Get is called by gRPC when your client initiates a request. Up will return
// find the least used address and return that for a client to use. If
// BlockingWait is true on the BalancerGetOptions this method will block until
// an address is available, otherwise an error will be returned if no address
// have any available capacity.
func (lb *ThroughputLoadBalancer) Get(ctx context.Context, opts grpc.BalancerGetOptions) (grpc.Address, func(), error) {
	addr, err := lb.next(opts.BlockingWait)
	if err != nil {
		return grpc.Address{}, func() {}, err
	}

	r := func() {
		lb.mu.Lock()
		addr.release()
		lb.mu.Unlock()
	}

	return addr.Address, r, nil
}

// Notify returns a channel that is used to notify gRPC of changes to the
// address set.
func (lb *ThroughputLoadBalancer) Notify() <-chan []grpc.Address {
	return lb.notify
}

// Close is called by gRPC when a connection is being closed. For this load
// balancer it is a no-op.
func (*ThroughputLoadBalancer) Close() error {
	return nil
}

func (lb *ThroughputLoadBalancer) sendNotify() {
	grpcAddrs := make([]grpc.Address, len(lb.addrs))
	for i, a := range lb.addrs {
		grpcAddrs[i] = a.Address
	}

	lb.notify <- grpcAddrs
}

// Finds the next address that is in an up state with the lowest active
// requests.
func (lb *ThroughputLoadBalancer) next(wait bool) (*address, error) {
	for {
		var addr *address

		// Set initial lowest capacity to a number that is not reachable by
		// any addresses
		lowestCapacity := lb.maxRequests * 2

		lb.mu.Lock()
		for _, a := range lb.addrs {
			// If the address is in a down state or at maximum capacity,
			// continue to the next address.
			if a.isDown() || a.atCapacity() {
				continue
			}

			// If the capacity is less than the lowest capacity, set addr to
			// the address and set the current lowestCapacity to the addresses
			// capacity.
			if a.capacity() < lowestCapacity {
				addr = a
				lowestCapacity = a.capacity()
			}
		}

		// If we found an address that is available to use, claim the address.
		// If we successfully claim the address return it other wise continue.
		if addr != nil {
			err := addr.claim()
			if err == nil {
				lb.mu.Unlock()
				return addr, nil
			}
		}
		lb.mu.Unlock()

		if !wait {
			return nil, errUnavailable
		}

		time.Sleep(50 * time.Millisecond)
	}
}

type address struct {
	grpc.Address

	state          addrState
	activeRequests int
	maxRequests    int
}

func (a *address) claim() error {
	if a.activeRequests >= a.maxRequests {
		return errMaxRequestsExceeded
	}

	a.activeRequests++

	return nil
}

func (a *address) release() {
	a.activeRequests--
}

func (a *address) goUp() {
	a.state = stateUp
}

// TODO: As per the grpc-go documentation, it is not clear how to construct
// and take advantage of the meaningful error parameter for down. Need
// realistic demands to guide.
func (a *address) goDown(_ error) {
	a.state = stateDown
}

func (a *address) isDown() bool {
	return a.state == stateDown
}

func (a *address) capacity() int {
	return a.activeRequests
}

func (a *address) atCapacity() bool {
	return a.activeRequests >= a.maxRequests
}
