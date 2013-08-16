package loggregatoragent

import (
	"github.com/cloudfoundry/gosteno"
	"loggregatorclient"
	"net"
)

type agent struct {
	*gosteno.Logger
	unixSocketPath string
}

func NewAgent(unixSocketPath string, logger *gosteno.Logger) agent {
	return agent{logger, unixSocketPath}
}

func (a agent) Start(loggregatorClient loggregatorclient.LoggregatorClient) {
	addr, err := net.ResolveUnixAddr("unixgram", a.unixSocketPath)
	if err != nil {
		panic(err)
	}
	socketListener, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		panic(err)
	}
	defer socketListener.Close()
	for {
		b := make([]byte, 65535) //buffer with size = max theoretical UDP size
		readCount, _, err := socketListener.ReadFrom(b)
		if err != nil {
			a.Debugf("Error reading from socket %s retrying, err: %v", a.unixSocketPath, err)
		}
		if readCount > 0 {
			loggregatorClient.Send(b[:readCount])
		}
	}
}
