package clientreader

import (
	"doppler/dopplerservice"
	"fmt"
)

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	SetAddresses(addresses []string) int
}

func Read(clientPool ClientPool, preferredProtocol string, event dopplerservice.Event) {

	clients := 0
	switch preferredProtocol {
	case "udp":
		clients = clientPool.SetAddresses(event.UDPDopplers)
	case "tcp":
		clients = clientPool.SetAddresses(event.TCPDopplers)
	case "tls":
		clients = clientPool.SetAddresses(event.TLSDopplers)
	}

	if clients == 0 {
		panic(fmt.Sprintf("No %s enabled dopplers available, check your manifest to make sure you have dopplers listening for %[1]s", preferredProtocol))
	}
}
