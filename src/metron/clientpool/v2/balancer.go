package v2

import (
	"fmt"
	"math/rand"
	"net"
)

// Balancer provides IPs resolved from a DNS address in random order
type Balancer struct {
	addr   string
	lookup func(string) ([]net.IP, error)
}

// BalancerOption is a type that will manipulate a config
type BalancerOption func(*Balancer)

// WithLookup sets the behavior of looking up IPs
func WithLookup(lookup func(string) ([]net.IP, error)) func(*Balancer) {
	return func(b *Balancer) {
		b.lookup = lookup
	}
}

// NewBalancer returns a Balancer
func NewBalancer(addr string, opts ...BalancerOption) *Balancer {
	balancer := &Balancer{
		addr:   addr,
		lookup: net.LookupIP,
	}

	for _, o := range opts {
		o(balancer)
	}

	return balancer
}

// NextHostPort returns hostport resolved from the balancer's addr.
// It returns error for an invalid addr or if lookup failed or
// doesn't resolve to anything.
func (b *Balancer) NextHostPort() (string, error) {
	host, port, err := net.SplitHostPort(b.addr)
	if err != nil {
		return "", err
	}

	ips, err := b.lookup(host)
	if err != nil {
		return "", err
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("lookup failed with addr %s", b.addr)
	}

	return net.JoinHostPort(ips[rand.Int()%len(ips)].String(), port), nil

}
