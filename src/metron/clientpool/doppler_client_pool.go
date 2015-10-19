package clientpool

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
)

var ErrorEmptyClientPool = errors.New("loggregator client pool is empty")

type DopplerPool struct {
	logger        *gosteno.Logger
	clientFactory func(logger *gosteno.Logger, url string) (loggregatorclient.Client, error)

	sync.RWMutex
	clients    map[string]loggregatorclient.Client
	clientList []loggregatorclient.Client

	nonLegacyServers map[string]string
	legacyServers    map[string]string
}

func NewDopplerPool(logger *gosteno.Logger, clientFactory func(logger *gosteno.Logger, url string) (loggregatorclient.Client, error)) *DopplerPool {
	return &DopplerPool{
		logger:        logger,
		clientFactory: clientFactory,
	}
}

func (pool *DopplerPool) Set(all map[string]string, preferred map[string]string) {
	pool.Lock()

	if len(preferred) > 0 {
		pool.nonLegacyServers = preferred
	} else if len(all) > 0 {
		pool.nonLegacyServers = all
	} else {
		pool.nonLegacyServers = nil
	}

	pool.merge()
	pool.Unlock()
}

func (pool *DopplerPool) SetLegacy(all map[string]string, preferred map[string]string) {
	pool.Lock()

	if len(preferred) > 0 {
		pool.legacyServers = preferred
	} else if len(all) > 0 {
		pool.legacyServers = all
	} else {
		pool.legacyServers = nil
	}

	pool.merge()
	pool.Unlock()
}

func (pool *DopplerPool) Clients() []loggregatorclient.Client {
	defer pool.RUnlock()
	pool.RLock()
	return pool.clientList
}

func (pool *DopplerPool) RandomClient() (loggregatorclient.Client, error) {
	list := pool.Clients()

	if len(list) == 0 {
		return nil, ErrorEmptyClientPool
	}

	return list[rand.Intn(len(list))], nil
}

func (pool *DopplerPool) getClient(url string) loggregatorclient.Client {
	var client loggregatorclient.Client
	if client = pool.clients[url]; client == nil {
		return nil
	}

	// scheme://address
	if !((len(url) == len(client.Scheme())+3+len(client.Address())) &&
		strings.HasPrefix(url, client.Scheme()) && strings.HasSuffix(url, client.Address())) {
		return nil
	}

	return client
}

func (pool *DopplerPool) merge() {
	newClients := map[string]loggregatorclient.Client{}

	for key, u := range pool.nonLegacyServers {
		client := pool.getClient(u)
		if client == nil {
			var err error
			client, err = pool.clientFactory(pool.logger, u)
			if err != nil {
				pool.logger.Errord(map[string]interface{}{
					"doppler": key, "url": u, "error": err,
				}, "Invalid url")
				continue
			}
		}
		newClients[key] = client
	}

	for key, u := range pool.legacyServers {
		if _, ok := newClients[key]; !ok {
			client := pool.getClient(u)
			if client == nil {
				var err error
				client, err = pool.clientFactory(pool.logger, u)
				if err != nil {
					pool.logger.Errord(map[string]interface{}{
						"doppler": key, "url": u, "error": err,
					}, "Invalid url")
					continue
				}
			}
			newClients[key] = client
		}
	}

	for address, client := range pool.clients {
		if _, ok := newClients[address]; !ok {
			client.Stop()
		}
	}

	newList := make([]loggregatorclient.Client, 0, len(newClients))
	for _, client := range newClients {
		newList = append(newList, client)
	}

	pool.clients = newClients
	pool.clientList = newList
}

func NewClient(logger *gosteno.Logger, url string) (loggregatorclient.Client, error) {
	if index := strings.Index(url, "://"); index > 0 {
		switch url[:index] {
		case "udp":
			return loggregatorclient.NewUDPClient(logger, url[index+3:], loggregatorclient.DefaultBufferSize)
		case "tls":
			return loggregatorclient.NewTLSClient(logger, url[index+3:])
		}
	}

	return nil, fmt.Errorf("Unknown scheme for %s", url)
}
