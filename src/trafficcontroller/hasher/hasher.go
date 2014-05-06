package hasher

import (
	"math/big"
	"trafficcontroller/listener"
)

var NewWebsocketListener = func(appId string) listener.Listener {
	return listener.NewWebsocket(appId)
}

type Hasher interface {
	LoggregatorServers() []string
	GetLoggregatorServerForAppId(string) string
	ProxyMessagesFor(string, listener.OutputChannel, listener.StopChannel) error
}

type hasher struct {
	items []string
}

func NewHasher(loggregatorServers []string) Hasher {
	if len(loggregatorServers) == 0 {
		panic("Hasher must be seeded with one or more Loggregator Servers")
	}

	return &hasher{items: loggregatorServers}
}

func (h *hasher) GetLoggregatorServerForAppId(appId string) string {
	var id, numberOfItems big.Int
	id.SetBytes([]byte(appId))
	numberOfItems.SetInt64(int64(len(h.items)))

	id.Mod(&id, &numberOfItems)
	return h.items[id.Int64()]
}

func (h *hasher) LoggregatorServers() []string {
	return h.items
}

func (h *hasher) ProxyMessagesFor(appId string, outgoing listener.OutputChannel, stop listener.StopChannel) error {
	l := NewWebsocketListener(appId)
	serverAddress := h.GetLoggregatorServerForAppId(appId)
	return l.Start(serverAddress, outgoing, stop)
}
