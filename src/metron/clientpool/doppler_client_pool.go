package clientpool

import (
	"errors"
	"sync"

	"metron/writers/dopplerforwarder"

	"math/rand"

	"github.com/cloudfoundry/gosteno"
)

var ErrorEmptyClientPool = errors.New("loggregator client pool is empty")

//go:generate hel --type Client --output mock_client_test.go

type Client interface {
	Scheme() string
	Address() string

	// Write implements dopplerforwarder.Client
	Write(message []byte) (bytesSent int, err error)

	// Close implements dopplerforwarder.Client
	Close() error
}

//go:generate hel --type ClientCreator --output mock_client_creator_test.go

type ClientCreator interface {
	CreateClient(url string) (client Client, err error)
}

type DopplerPool struct {
	logger *gosteno.Logger

	sync.RWMutex
	clients []Client

	clientCreator ClientCreator
}

func NewDopplerPool(logger *gosteno.Logger, clientCreator ClientCreator) *DopplerPool {
	return &DopplerPool{
		logger:        logger,
		clientCreator: clientCreator,
	}
}

func (pool *DopplerPool) SetAddresses(addresses []string) {
	pool.clients = make([]Client, 0, len(addresses))
	for _, address := range addresses {
		client, err := pool.clientCreator.CreateClient(address)
		if err != nil {
			pool.logger.Errorf("Failed to connect to client at %s: %v", address, err)
			continue
		}
		pool.clients = append(pool.clients, client)
	}
}

func (pool *DopplerPool) Clients() []Client {
	pool.RLock()
	defer pool.RUnlock()

	clientList := make([]Client, len(pool.clients))
	copy(clientList, pool.clients)
	return clientList
}

// RandomClient implements dopplerforwarder.DopplerPool
func (pool *DopplerPool) RandomClient() (dopplerforwarder.Client, error) {
	list := pool.Clients()

	if len(list) == 0 {
		return nil, ErrorEmptyClientPool
	}

	return list[rand.Intn(len(list))], nil
}
