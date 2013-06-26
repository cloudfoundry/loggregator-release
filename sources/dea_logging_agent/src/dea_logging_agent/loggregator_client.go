package dea_logging_agent

import "net"

type LoggregatorClient interface {
	Send([]byte)
}

type TcpLoggregatorClient struct {
	Config *Config
	conn   net.Conn
}

func (loggregatorClient *TcpLoggregatorClient) Send(data []byte) {
	openConnection := func() net.Conn {
		if loggregatorClient.conn == nil {
			conn, err := net.Dial("tcp", config.LoggregatorAddress)
			loggregatorClient.conn = conn
			if err != nil {
				logger.Fatalf("Dialing to loggregator %s failed %e", config.LoggregatorAddress, err)
				panic(err)
			}
		}
		return loggregatorClient.conn
	}

	writeCount, err := openConnection().Write(data)
	logger.Debugf("Wrote %i bytes to %s", writeCount, config.LoggregatorAddress)
	if err != nil {
		logger.Fatalf("Writing to loggregator %s failed %e", config.LoggregatorAddress, err)
		panic(err)
	}
}
