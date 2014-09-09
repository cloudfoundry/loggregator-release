package channel_group_connector

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"sync"
	"time"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/serveraddressprovider"
)

const checkServerAddressesInterval = 100 * time.Millisecond

type ListenerConstructor func() listener.Listener

type ChannelGroupConnector interface {
	Connect(path string, appId string, messagesChan chan<- []byte, stopChan <-chan struct{}, reconnect bool)
}

type channelGroupConnector struct {
	serverAddressProvider serveraddressprovider.ServerAddressProvider
	logger                *gosteno.Logger
	listenerConstructor   ListenerConstructor
	generateLogMessage    marshaller.MessageGenerator
}

func NewChannelGroupConnector(provider serveraddressprovider.ServerAddressProvider, listenerConstructor ListenerConstructor, logMessageGenerator marshaller.MessageGenerator, logger *gosteno.Logger) ChannelGroupConnector {
	return &channelGroupConnector{
		serverAddressProvider: provider,
		listenerConstructor:   listenerConstructor,
		generateLogMessage:    logMessageGenerator,
		logger:                logger,
	}
}

func (connector *channelGroupConnector) Connect(path string, appId string, messagesChan chan<- []byte, stopChan <-chan struct{}, reconnect bool) {
	defer close(messagesChan)
	connections := &serverConnections{
		connectedAddresses: make(map[string]struct{}),
	}

	checkLoggregatorServersTicker := time.NewTicker(checkServerAddressesInterval)
	defer checkLoggregatorServersTicker.Stop()

loop:
	for {
		serverAddresses := connector.serverAddressProvider.ServerAddresses()

		for _, serverAddress := range serverAddresses {
			if connections.connectedToServer(serverAddress) {
				continue
			}
			connections.addConnectedServer(serverAddress)

			go func(addr string) {
				connector.connectToServer(addr, path, appId, messagesChan, stopChan)
				connections.removeConnectedServer(addr)
			}(serverAddress)
		}

		if !reconnect {
			break
		}

		select {
		case <-checkLoggregatorServersTicker.C:
		case <-stopChan:
			break loop
		}

	}

	connections.Wait()
}

func (connector *channelGroupConnector) connectToServer(serverAddress string, path string, appId string, messagesChan chan<- []byte, stopChan <-chan struct{}) {
	l := connector.listenerConstructor()
	serverUrlForAppId := fmt.Sprintf("ws://%s/apps/%s%s", serverAddress, appId, path)

	err := l.Start(serverUrlForAppId, appId, messagesChan, stopChan)

	if err != nil {
		errorMsg := fmt.Sprintf("proxy: error connecting to %s: %s", serverAddress, err.Error())
		messagesChan <- connector.generateLogMessage(errorMsg, appId)
		connector.logger.Errorf("proxy: error connecting %s %s %s", appId, path, err.Error())
	}
}

type serverConnections struct {
	connectedAddresses map[string]struct{}
	sync.Mutex
	sync.WaitGroup
}

func (connections *serverConnections) connectedToServer(serverAddress string) bool {
	connections.Lock()
	defer connections.Unlock()

	_, connected := connections.connectedAddresses[serverAddress]
	return connected
}

func (connections *serverConnections) addConnectedServer(serverAddress string) {
	connections.Lock()
	defer connections.Unlock()

	connections.Add(1)
	connections.connectedAddresses[serverAddress] = struct{}{}
}

func (connections *serverConnections) removeConnectedServer(serverAddress string) {
	connections.Lock()
	defer connections.Unlock()
	defer connections.Done()

	delete(connections.connectedAddresses, serverAddress)
}
