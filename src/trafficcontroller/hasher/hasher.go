package hasher

import (
	"math/big"
)

type Hasher interface {
	LoggregatorServers() []string
	GetLoggregatorServerForAppId(string) string
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
