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
	IncLogStreamRawByteCount(uint64)
	IncLogStreamPbByteCount(uint64)
}

type udpLoggregatorClient struct {
	receivedMessageCount  *uint64
	sentMessageCount      *uint64
	receivedByteCount     *uint64
	sentByteCount         *uint64
	logStreamRawByteCount *uint64
	logStreamPbByteCount  *uint64
	sendChannel           chan []byte
}

func NewLoggregatorClient(loggregatorAddress string, logger *gosteno.Logger, bufferSize int) LoggregatorClient {
	loggregatorClient := &udpLoggregatorClient{receivedMessageCount: new(uint64), sentMessageCount: new(uint64),
		receivedByteCount: new(uint64), sentByteCount: new(uint64), logStreamRawByteCount: new(uint64),
		logStreamPbByteCount: new(uint64)}

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
			instrumentation.Metric{"currentBufferCount", uint64(len(loggregatorClient.sendChannel))},
			instrumentation.Metric{"sentMessageCount", atomic.LoadUint64(loggregatorClient.sentMessageCount)},
			instrumentation.Metric{"receivedMessageCount", atomic.LoadUint64(loggregatorClient.receivedMessageCount)},
			instrumentation.Metric{"sentByteCount", atomic.LoadUint64(loggregatorClient.sentByteCount)},
			instrumentation.Metric{"receivedByteCount", atomic.LoadUint64(loggregatorClient.receivedByteCount)},
			instrumentation.Metric{"logStreamRawByteCount", atomic.LoadUint64(loggregatorClient.logStreamRawByteCount)},
			instrumentation.Metric{"logStreamPbByteCount", atomic.LoadUint64(loggregatorClient.logStreamPbByteCount)},
		},
	}
}

func (loggregatorClient *udpLoggregatorClient) IncLogStreamRawByteCount(count uint64) {
	atomic.AddUint64(loggregatorClient.logStreamRawByteCount, count)
}

func (loggregatorClient *udpLoggregatorClient) IncLogStreamPbByteCount(count uint64) {
	atomic.AddUint64(loggregatorClient.logStreamPbByteCount, count)
}
