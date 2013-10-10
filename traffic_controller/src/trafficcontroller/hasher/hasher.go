package hasher

import (
	"math/big"
)

type Hasher struct {
	items []string
}

func NewHasher(loggregatorServers []string) (h *Hasher) {
	if len(loggregatorServers) == 0 {
		panic("Hasher must be seeded with one or more Loggregator Servers")
	}

	h = &Hasher{items: loggregatorServers}
	return
}

func (h *Hasher) GetLoggregatorServerForAppId(appId string) string {
	var id, numberOfItems big.Int
	id.SetBytes([]byte(appId))
	numberOfItems.SetInt64(int64(len(h.items)))

	id.Mod(&id, &numberOfItems)
	return h.items[id.Int64()]
}

func (h *Hasher) LoggregatorServers() []string {
	return h.items
}
