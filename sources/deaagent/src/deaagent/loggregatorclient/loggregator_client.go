package loggregatorclient

import (
	"github.com/cloudfoundry/gosteno"
	"instrumentor"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type LoggregatorClient interface {
	Send([]byte)
}

type udpLoggregatorClient struct {
	receivedMessageCount *uint64
	sentMessageCount     *uint64
	sendChannel          chan []byte
}

func NewLoggregatorClient(loggregatorAddress string, logger *gosteno.Logger, bufferSize int) LoggregatorClient {
	loggregatorClient := &udpLoggregatorClient{receivedMessageCount: new(uint64), sentMessageCount: new(uint64)}

	lcInstrumentor := instrumentor.NewInstrumentor(5*time.Second, gosteno.LOG_DEBUG, logger)
	stopChan := lcInstrumentor.Instrument(loggregatorClient)

	connection, err := net.Dial("udp", loggregatorAddress)
	if err != nil {
		logger.Fatalf("Error resolving loggregator address %s, %s", loggregatorAddress, err)
		panic(err)
	}

	loggregatorClient.sendChannel = make(chan []byte, bufferSize)

	go func() {
		defer lcInstrumentor.StopInstrumentation(stopChan)
		for {
			dataToSend := <-loggregatorClient.sendChannel
			if len(dataToSend) > 0 {
				writeCount, err := connection.Write(dataToSend)
				logger.Debugf("Wrote %d bytes to %s", writeCount, loggregatorAddress)
				atomic.AddUint64(loggregatorClient.sentMessageCount, 1)
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
	loggregatorClient.sendChannel <- data
}

func (loggregatorClient *udpLoggregatorClient) DumpData() []instrumentor.PropVal {

	return []instrumentor.PropVal{
		instrumentor.PropVal{"CurrentBufferCount", strconv.Itoa(len(loggregatorClient.sendChannel))},
		instrumentor.PropVal{"ReceivedMessageCount", strconv.FormatUint(atomic.LoadUint64(loggregatorClient.receivedMessageCount), 10)},
		instrumentor.PropVal{"SentMessageCount", strconv.FormatUint(atomic.LoadUint64(loggregatorClient.sentMessageCount), 10)},
	}
}
