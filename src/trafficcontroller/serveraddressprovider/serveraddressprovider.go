package serveraddressprovider

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"time"
)

type ServerAddressProvider interface {
	ServerAddresses() []string
	Start()
}

func NewDynamicServerAddressProvider(serverAddressList servicediscovery.ServerAddressList, port uint32, interval time.Duration) ServerAddressProvider {
	return &dynamicServerAddressProvider{
		serverAddressList: serverAddressList,
		port:              port,
		interval:          interval,
	}
}

type dynamicServerAddressProvider struct {
	serverAddressList servicediscovery.ServerAddressList
	port              uint32
	interval          time.Duration
}

func (provider *dynamicServerAddressProvider) Start() {
	provider.serverAddressList.DiscoverAddresses()
	go provider.serverAddressList.Run(provider.interval)
}

func (provider *dynamicServerAddressProvider) ServerAddresses() []string {
	addrsWithPort := []string{}

	for _, addr := range provider.serverAddressList.GetAddresses() {
		addrsWithPort = append(addrsWithPort, fmt.Sprintf("%s:%d", addr, provider.port))
	}

	return addrsWithPort
}
