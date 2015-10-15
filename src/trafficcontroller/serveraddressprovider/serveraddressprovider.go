package serveraddressprovider

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
)

type ServerAddressProvider interface {
	ServerAddresses() []string
	Start()
}

func NewDynamicServerAddressProvider(serverAddressList servicediscovery.ServerAddressList, port uint32) ServerAddressProvider {
	return &dynamicServerAddressProvider{
		serverAddressList: serverAddressList,
		port:              port,
	}
}

type dynamicServerAddressProvider struct {
	serverAddressList servicediscovery.ServerAddressList
	port              uint32
	interval          time.Duration
}

func (provider *dynamicServerAddressProvider) Start() {
	provider.serverAddressList.DiscoverAddresses()
	go provider.serverAddressList.Run()
}

func (provider *dynamicServerAddressProvider) ServerAddresses() []string {
	addrsWithPort := []string{}

	for _, addr := range provider.serverAddressList.GetAddresses() {
		addrsWithPort = append(addrsWithPort, fmt.Sprintf("%s:%d", addr, provider.port))
	}

	return addrsWithPort
}
