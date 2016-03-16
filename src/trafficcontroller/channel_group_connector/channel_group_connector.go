package channel_group_connector

import (
	"fmt"
	"sync"
	"time"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/listener"
	"trafficcontroller/marshaller"

	"github.com/cloudfoundry/gosteno"
)

const checkServerAddressesInterval = 100 * time.Millisecond

type ListenerConstructor func(time.Duration, *gosteno.Logger) listener.Listener

//go:generate hel --type Finder --output mock_finder_test.go

type Finder interface {
	WebsocketServers() []string
}

type ChannelGroupConnector struct {
	finder              Finder
	logger              *gosteno.Logger
	listenerConstructor ListenerConstructor
	generateLogMessage  marshaller.MessageGenerator
}

func NewChannelGroupConnector(finder Finder, listenerConstructor ListenerConstructor, logMessageGenerator marshaller.MessageGenerator, logger *gosteno.Logger) *ChannelGroupConnector {
	return &ChannelGroupConnector{
		finder:              finder,
		listenerConstructor: listenerConstructor,
		generateLogMessage:  logMessageGenerator,
		logger:              logger,
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
		serverURLs := c.finder.WebsocketServers()
		if len(serverURLs) == 0 {
			c.logger.Debugf("ChannelGroupConnector.Connect: No doppler servers available. Trying again in %s", checkServerAddressesInterval.String())
		} else {
			for _, serverURL := range serverURLs {
				if connections.connectedToServer(serverURL) {
					continue
				}
				connections.addConnectedServer(serverURL)

				go func(addr string) {
					c.connectToServer(addr, dopplerEndpoint, messagesChan, stopChan)
					connections.removeConnectedServer(addr)
				}(serverURL)
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
	c.logger.Infof("proxy: connecting to doppler at %s", serverUrl)

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
