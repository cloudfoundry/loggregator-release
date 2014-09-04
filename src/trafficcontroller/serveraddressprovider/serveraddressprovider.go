package serveraddressprovider

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
)

type ServerAddressProvider interface {
	ServerAddresses() []string
}

func NewDynamicServerAddressProvider(serverAddressList servicediscovery.ServerAddressList, port uint32) ServerAddressProvider {
	return &dynamicServerAddressProvider{serverAddressList: serverAddressList, port: port}
}

type dynamicServerAddressProvider struct {
	serverAddressList servicediscovery.ServerAddressList
	port              uint32
}

func (provider *dynamicServerAddressProvider) ServerAddresses() []string {
	addrsWithPort := []string{}

	for _, addr := range provider.serverAddressList.GetAddresses() {
		addrsWithPort = append(addrsWithPort, fmt.Sprintf("%s:%d", addr, provider.port))
	}

	return addrsWithPort
}
