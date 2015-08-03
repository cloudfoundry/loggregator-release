package serveraddressprovider

import "sync"

type FakeServerAddressProvider struct {
	serverAddresses []string
	callCount       int
	sync.Mutex
}

func (p *FakeServerAddressProvider) CallCount() int {
	p.Lock()
	defer p.Unlock()
	return p.callCount
}

func (p *FakeServerAddressProvider) SetServerAddresses(addresses []string) {
	p.Lock()
	defer p.Unlock()
	p.serverAddresses = addresses
}

func (p *FakeServerAddressProvider) ServerAddresses() []string {
	p.Lock()
	defer p.Unlock()
	p.callCount += 1
	return p.serverAddresses
}

func (p *FakeServerAddressProvider) DiscoverAddresses() {

}

func (p *FakeServerAddressProvider) Start() {}
