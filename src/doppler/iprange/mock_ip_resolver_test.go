package iprange_test

import "net"

type mockIPResolver struct {
	ResolveIPAddrCalled chan bool
	ResolveIPAddrInput  struct {
		net, addr chan string
	}
	ResolveIPAddrOutput struct {
		ret0 chan *net.IPAddr
		ret1 chan error
	}
}

func newMockIPResolver() *mockIPResolver {
	m := &mockIPResolver{}
	m.ResolveIPAddrCalled = make(chan bool, 100)
	m.ResolveIPAddrInput.net = make(chan string, 100)
	m.ResolveIPAddrInput.addr = make(chan string, 100)
	m.ResolveIPAddrOutput.ret0 = make(chan *net.IPAddr, 100)
	m.ResolveIPAddrOutput.ret1 = make(chan error, 100)
	return m
}
func (m *mockIPResolver) ResolveIPAddr(net, addr string) (*net.IPAddr, error) {
	m.ResolveIPAddrCalled <- true
	m.ResolveIPAddrInput.net <- net
	m.ResolveIPAddrInput.addr <- addr
	return <-m.ResolveIPAddrOutput.ret0, <-m.ResolveIPAddrOutput.ret1
}
