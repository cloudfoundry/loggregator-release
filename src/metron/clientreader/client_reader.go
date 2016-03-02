package clientreader

import "doppler/dopplerservice"

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	SetAddresses(addresses []string) int
}

func Read(clientPool ClientPool, preferredProtocol string, event dopplerservice.Event) {

	clients := 0
	switch preferredProtocol {
	case "tls":
		clients = clientPool.SetAddresses(event.TLSDopplers)
	case "udp":
		clients = clientPool.SetAddresses(event.UDPDopplers)
	}

	if clients == 0 {
		panic("No available dopplers")
	}
}
