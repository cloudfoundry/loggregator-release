package loggregatorclient

import (
	"github.com/cloudfoundry/gosteno"
	"net"
)

type LoggregatorClient interface {
	Send([]byte)
}

type udpLoggregatorClient struct {
	sendChannel chan []byte
}

func NewLoggregatorClient(loggregatorAddress string, logger *gosteno.Logger, bufferSize int) *udpLoggregatorClient {
	loggregatorClient := &udpLoggregatorClient{}

	connection, err := net.Dial("udp", loggregatorAddress)
	if err != nil {
		logger.Fatalf("Error resolving loggregator address %s, %s", loggregatorAddress, err)
		panic(err)
	}

	loggregatorClient.sendChannel = make(chan []byte, bufferSize)

	go func() {
		for {
			dataToSend := <-loggregatorClient.sendChannel
			if len(dataToSend) > 0 {
				writeCount, err := connection.Write(dataToSend)
				logger.Debugf("Wrote %d bytes to %s", writeCount, loggregatorAddress)
				if err != nil {
					logger.Errorf("Writing to loggregator %s failed %s", loggregatorAddress, err)
				}
			} else {
				logger.Debugf("Skipped writing of 0 byte message to %s", loggregatorAddress)
			}
		}
	}()

	return loggregatorClient
}

func (loggregatorClient *udpLoggregatorClient) Send(data []byte) {
	loggregatorClient.sendChannel <- data
}
