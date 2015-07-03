package channel_group_connector

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"sync"
	"time"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"
	"trafficcontroller/serveraddressprovider"
)

const checkServerAddressesInterval = 100 * time.Millisecond

type ListenerConstructor func(time.Duration, *gosteno.Logger) listener.Listener

type ChannelGroupConnector struct {
	serverAddressProvider serveraddressprovider.ServerAddressProvider
	logger                *gosteno.Logger
	listenerConstructor   ListenerConstructor
	generateLogMessage    marshaller.MessageGenerator
}

func NewChannelGroupConnector(provider serveraddressprovider.ServerAddressProvider, listenerConstructor ListenerConstructor, logMessageGenerator marshaller.MessageGenerator, logger *gosteno.Logger) *ChannelGroupConnector {
	return &ChannelGroupConnector{
		serverAddressProvider: provider,
		listenerConstructor:   listenerConstructor,
		generateLogMessage:    logMessageGenerator,
		logger:                logger,
	}
}

func (c *ChannelGroupConnector) Connect(dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{}) {
	defer close(messagesChan)
	connections := &serverConnections{
		connectedAddresses: make(map[string]struct{}),
	}

	checkLoggregatorServersTicker := time.NewTicker(checkServerAddressesInterval)
	defer checkLoggregatorServersTicker.Stop()

loop:
	for {
		serverAddresses := c.serverAddressProvider.ServerAddresses()

		if len(serverAddresses) == 0 {
			c.logger.Debugf("ChannelGroupConnector.Connect: No doppler servers available. Trying again in %s", checkServerAddressesInterval.String())
		} else {
			for _, serverAddress := range serverAddresses {
				if connections.connectedToServer(serverAddress) {
					continue
				}
				connections.addConnectedServer(serverAddress)

				go func(addr string) {
					c.connectToServer(addr, dopplerEndpoint, messagesChan, stopChan)
					connections.removeConnectedServer(addr)
				}(serverAddress)
			}
		}

		if !dopplerEndpoint.Reconnect {
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

func (c *ChannelGroupConnector) connectToServer(serverAddress string, dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{}) {
	l := c.listenerConstructor(dopplerEndpoint.Timeout, c.logger)

	serverUrl := fmt.Sprintf("ws://%s%s", serverAddress, dopplerEndpoint.GetPath())
	c.logger.Debugf("proxy: connecting to doppler at %s", serverUrl)

	appId := dopplerEndpoint.StreamId
	err := l.Start(serverUrl, appId, messagesChan, stopChan)

	if err != nil {
		errorMsg := fmt.Sprintf("proxy: error connecting to %s: %s", serverAddress, err.Error())
		messagesChan <- c.generateLogMessage(errorMsg, appId)
		c.logger.Errorf("proxy: error connecting %s %s %s", appId, dopplerEndpoint.Endpoint, err.Error())
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
