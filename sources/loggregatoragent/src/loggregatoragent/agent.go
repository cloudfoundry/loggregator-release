package loggregatoragent

import (
	"github.com/cloudfoundry/gosteno"
	"loggregatorclient"
	"net"
	"os"
)

type agent struct {
	*gosteno.Logger
	unixSocketPath string
	KillChan       chan bool
}

func NewAgent(unixSocketPath string, logger *gosteno.Logger) agent {
	return agent{logger, unixSocketPath, make(chan bool)}
}

func (a agent) Start(loggregatorClient loggregatorclient.LoggregatorClient) {

	addr, err := net.ResolveUnixAddr("unixgram", a.unixSocketPath)
	if err != nil {
		a.Logger.Errorf("Error resolving Unix address from socket path %s", a.unixSocketPath)
		a.KillChan <- true
		return
	}
	socketListener, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		a.Logger.Warnf("Could not listen to socket at address %s, retrying.", a.unixSocketPath)
		os.Remove(a.unixSocketPath)
		socketListener, err = net.ListenUnixgram("unixgram", addr)
		if err != nil {
			a.Logger.Errorf("Error listening to socket at address %s", a.unixSocketPath)
			a.KillChan <- true
			return
		}
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
