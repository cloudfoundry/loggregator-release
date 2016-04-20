package clientreader

import (
	"doppler/dopplerservice"
	"fmt"
)

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	SetAddresses(addresses []string) int
}

func Read(clientPool map[string]ClientPool, protocols []string, event dopplerservice.Event) {
	var (
		servers  []string
		protocol string
	)

loop:
	for _, protocol = range protocols {
		switch protocol {
		case "udp":
			servers = event.UDPDopplers
			if len(servers) > 0 {
				break loop
			}
		case "tcp":
			servers = event.TCPDopplers
			if len(servers) > 0 {
				break loop
			}
		case "tls":
			servers = event.TLSDopplers
			if len(servers) > 0 {
				break loop
			}
		}
	}

	clients := clientPool[protocol].SetAddresses(servers)

	if clients == 0 {
		panic(fmt.Sprintf("No enabled dopplers available, check your manifest to make sure you have dopplers listening for the following protocols %v", protocols))
	}
}
