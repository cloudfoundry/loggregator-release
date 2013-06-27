package dea_logging_agent

import (
	"net"
	"sync"
)

type LoggregatorClient interface {
	Send([]byte)
}

type TcpLoggregatorClient struct {
	sendChannel          chan *[]byte
	sendChannelMutex     sync.Mutex
}

func (loggregatorClient *TcpLoggregatorClient) Send(data []byte) {
	loggregatorClient.dataChannel() <- &data
}


func (loggregatorClient *TcpLoggregatorClient) dataChannel() (chan *[]byte) {
	loggregatorClient.sendChannelMutex.Lock()
	defer loggregatorClient.sendChannelMutex.Unlock()

	if (loggregatorClient.sendChannel != nil) {
		return loggregatorClient.sendChannel
	}

	loggregatorClient.sendChannel = make(chan *[]byte, bufferSize)

	connection, err := net.Dial("tcp", config.LoggregatorAddress)
	if err != nil {
		logger.Fatalf("Dialing to loggregator %s failed %e", config.LoggregatorAddress, err)
		panic(err)
	}

	go func() {
		for {
			dataToSend := <-loggregatorClient.sendChannel
			writeCount, err := connection.Write(*dataToSend)
			logger.Debugf("Wrote %d bytes to %s", writeCount, config.LoggregatorAddress)
			if err != nil {
				logger.Fatalf("Writing to loggregator %s failed %e", config.LoggregatorAddress, err)
				panic(err)
			}
		}
	}()

	return loggregatorClient.sendChannel
}
