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
			conn, err := net.Dial("tcp", loggregatorClient.Config.LoggregatorAddress)
			loggregatorClient.conn = conn
			if err != nil {
				panic(err)
			}
		}
		return loggregatorClient.conn
	}

	_, err := openConnection().Write(data)
	if err != nil {
		panic(err)
	}
}
