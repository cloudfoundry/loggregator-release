package loggregatorrouter

import "github.com/stathat/consistent"

type hasher struct {
	c *consistent.Consistent
}

func NewHasher(loggregatorServers []string) (h *hasher) {
	if len(loggregatorServers) == 0 {
		panic("Hasher must be seeded with one or more Loggregator Servers")
	}

	c := consistent.New()
	c.Set(loggregatorServers)

	h = &hasher{c: c}
	return
}

func (h *hasher) getLoggregatorServerForAppId(appId string) (string, error) {
	return h.c.Get(appId)
}

func (h *hasher) loggregatorServers() []string {
	return h.c.Members()
}
