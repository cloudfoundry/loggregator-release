package loggregatorclient

import (
	"cfcomponent/instrumentation"
	"github.com/cloudfoundry/gosteno"
	"net"
	"sync/atomic"
)

type LoggregatorClient interface {
	instrumentation.Instrumentable
	Send([]byte)
}

type udpLoggregatorClient struct {
	receivedMessageCount *uint64
	sentMessageCount     *uint64
	receivedByteCount    *uint64
	sentByteCount        *uint64
	sendChannel          chan []byte
}

func NewLoggregatorClient(loggregatorAddress string, logger *gosteno.Logger, bufferSize int) LoggregatorClient {
	loggregatorClient := &udpLoggregatorClient{receivedMessageCount: new(uint64), sentMessageCount: new(uint64),
		receivedByteCount: new(uint64), sentByteCount: new(uint64)}

	connection, err := net.Dial("udp", loggregatorAddress)
	if err != nil {
		logger.Fatalf("Error resolving loggregator address %s, %s", loggregatorAddress, err)
	}

	loggregatorClient.sendChannel = make(chan []byte, bufferSize)

	go func() {
		for {
			dataToSend := <-loggregatorClient.sendChannel
			if len(dataToSend) > 0 {
				writeCount, err := connection.Write(dataToSend)
				logger.Debugf("Wrote %d bytes to %s", writeCount, loggregatorAddress)
				atomic.AddUint64(loggregatorClient.sentMessageCount, 1)
				atomic.AddUint64(loggregatorClient.sentByteCount, uint64(writeCount))
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
	atomic.AddUint64(loggregatorClient.receivedMessageCount, 1)
	atomic.AddUint64(loggregatorClient.receivedByteCount, uint64(len(data)))
	loggregatorClient.sendChannel <- data
}

func (loggregatorClient *udpLoggregatorClient) Emit() instrumentation.Context {
	return instrumentation.Context{"loggregatorClient",
		[]instrumentation.Metric{
			instrumentation.Metric{"CurrentBufferCount", len(loggregatorClient.sendChannel)},
			instrumentation.Metric{"ReceivedMessageCount", atomic.LoadUint64(loggregatorClient.receivedMessageCount)},
			instrumentation.Metric{"SentMessageCount", atomic.LoadUint64(loggregatorClient.sentMessageCount)},
			instrumentation.Metric{"ReceivedByteCount", atomic.LoadUint64(loggregatorClient.receivedByteCount)},
			instrumentation.Metric{"SentByteCount", atomic.LoadUint64(loggregatorClient.sentByteCount)},
		},
	}
}
