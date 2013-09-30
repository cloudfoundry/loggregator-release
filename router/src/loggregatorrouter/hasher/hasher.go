package hasher

import "github.com/stathat/consistent"

type Hasher struct {
	c *consistent.Consistent
}

func NewHasher(loggregatorServers []string) (h *Hasher) {
	if len(loggregatorServers) == 0 {
		panic("Hasher must be seeded with one or more Loggregator Servers")
	}

	c := consistent.New()
	c.Set(loggregatorServers)

	h = &Hasher{c: c}
	return
}

func (h *Hasher) GetLoggregatorServerForAppId(appId string) (string, error) {
	return h.c.Get(appId)
}

func (h *Hasher) LoggregatorServers() []string {
	return h.c.Members()
}
