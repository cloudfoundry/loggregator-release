package deaagent

import (
	"net"
	"sync"
)

type LoggregatorClient interface {
	Send([]byte)
}

type UdpLoggregatorClient struct {
	sendChannel          chan []byte
	sendChannelMutex     sync.Mutex
}

func (loggregatorClient *UdpLoggregatorClient) Send(data []byte) {
	loggregatorClient.dataChannel() <- data
}


func (loggregatorClient *UdpLoggregatorClient) dataChannel() (chan []byte) {
	loggregatorClient.sendChannelMutex.Lock()
	defer loggregatorClient.sendChannelMutex.Unlock()

	if (loggregatorClient.sendChannel != nil) {
		return loggregatorClient.sendChannel
	}

	connection, err := net.Dial("udp", config.LoggregatorAddress)
	if err != nil {
		logger.Fatalf("Error resolving loggregator address %s, %s", config.LoggregatorAddress, err)
		panic(err)
	}

	loggregatorClient.sendChannel = make(chan []byte, bufferSize)

	go func() {
		for {
			dataToSend := <-loggregatorClient.sendChannel
			writeCount, err := connection.Write(dataToSend)
			logger.Debugf("Wrote %d bytes to %s", writeCount, config.LoggregatorAddress)
			if err != nil {
				logger.Errorf("Writing to loggregator %s failed %s", config.LoggregatorAddress, err)
			}
		}
	}()

	return loggregatorClient.sendChannel
}
