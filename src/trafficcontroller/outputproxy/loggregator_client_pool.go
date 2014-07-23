package outputproxy

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/storeadapter"
	"math/rand"
	"sync"
	"time"
)

var ErrorEmptyClientPool = errors.New("loggregator client pool is empty")

type LoggregatorClientPool struct {
	clients                 map[string]loggregatorclient.LoggregatorClient
	logger                  *gosteno.Logger
	createClientsEnabled    bool
	loggregatorIncomingPort int
	sync.RWMutex
}

func NewLoggregatorClientPool(logger *gosteno.Logger) *LoggregatorClientPool {
	return &LoggregatorClientPool{
		clients: make(map[string]loggregatorclient.LoggregatorClient),
		logger:  logger,
	}
}

func (pool *LoggregatorClientPool) EnableCreateClients(loggregatorIncomingPort int) {
	pool.createClientsEnabled = true
	pool.loggregatorIncomingPort = loggregatorIncomingPort
}

func (pool *LoggregatorClientPool) RandomClient() (loggregatorclient.LoggregatorClient, error) {
	list := pool.ListClients()
	if len(list) == 0 {
		return nil, ErrorEmptyClientPool
	}

	return list[rand.Intn(len(list))], nil
}

func (pool *LoggregatorClientPool) RunUpdateLoop(storeAdapter storeadapter.StoreAdapter, key string, stopChan <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			serverRoot, err := storeAdapter.ListRecursively(key)

			var nodes []storeadapter.StoreNode

			switch err {
			case nil:
				nodes = leafNodes(serverRoot)
			case storeadapter.ErrorKeyNotFound:
				nodes = []storeadapter.StoreNode{}
			default:
				pool.logger.Errorf("RunUpdateLoop: Error communicating with etcd: %s", err.Error())
				continue
			}

			pool.syncWithNodes(nodes)

		case <-stopChan:
			return
		}
	}
}

func (pool *LoggregatorClientPool) syncWithNodes(nodes []storeadapter.StoreNode) {
	pool.Lock()
	defer pool.Unlock()

	addressesToBeDeleted := make(map[string]bool)
	for addr := range pool.clients {
		addressesToBeDeleted[addr] = true
	}

	for _, node := range nodes {
		addr := string(node.Value)
		delete(addressesToBeDeleted, addr)

		if pool.hasServerFor(addr) {
			continue
		}

		serverAddr := fmt.Sprintf("%s:%d", addr, pool.loggregatorIncomingPort)
		var client loggregatorclient.LoggregatorClient
		if pool.createClientsEnabled {
			client = loggregatorclient.NewLoggregatorClient(serverAddr, pool.logger, loggregatorclient.DefaultBufferSize)
		}
		pool.clients[addr] = client
	}

	for addr := range addressesToBeDeleted {
		delete(pool.clients, addr)
	}
}

func (pool *LoggregatorClientPool) ListClients() []loggregatorclient.LoggregatorClient {
	pool.RLock()
	defer pool.RUnlock()

	val := make([]loggregatorclient.LoggregatorClient, 0, len(pool.clients))
	for _, client := range pool.clients {
		val = append(val, client)
	}

	return val
}

func (pool *LoggregatorClientPool) ListAddresses() []string {
	pool.RLock()
	defer pool.RUnlock()

	val := make([]string, 0, len(pool.clients))
	for addr := range pool.clients {
		val = append(val, addr)
	}

	return val
}

func (pool *LoggregatorClientPool) Add(address string, client loggregatorclient.LoggregatorClient) {
	pool.Lock()
	defer pool.Unlock()

	pool.clients[address] = client
}

func (pool *LoggregatorClientPool) hasServerFor(addr string) bool {
	_, ok := pool.clients[addr]
	return ok
}

func leafNodes(root storeadapter.StoreNode) []storeadapter.StoreNode {
	if !root.Dir {
		return []storeadapter.StoreNode{root}
	}

	leaves := []storeadapter.StoreNode{}
	for _, node := range root.ChildNodes {
		leaves = append(leaves, leafNodes(node)...)
	}
	return leaves
}
