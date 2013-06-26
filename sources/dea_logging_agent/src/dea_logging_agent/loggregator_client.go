package dea_logging_agent

import "net"

type LoggregatorClient interface {
	Send([]byte)
}

type TcpLoggregatorClient struct {
	config *Config
	conn net.Conn
}

func (loggregatorClient *TcpLoggregatorClient) connection() (net.Conn) {
	if loggregatorClient.conn == nil {
		conn, err := net.Dial("tcp", loggregatorClient.config.loggregatorAddress)
		loggregatorClient.conn = conn
		if err != nil {
			panic(err)
		}
	}
	return loggregatorClient.conn
}

func (loggregatorClient *TcpLoggregatorClient) Send(data []byte) {
	_, err := loggregatorClient.connection().Write(data)
	if err != nil {
		panic(err)
	}
}
