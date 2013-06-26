package dea_logging_agent

import (
	"net"
	"path/filepath"
	"runtime"
)

type Instance struct {
	ApplicationId          string
	WardenJobId            string
	WardenContainerPath    string
	listenerControlChannel chan (bool)
}

func (instance *Instance) Identifier() string {
	return filepath.Join(instance.WardenContainerPath, "jobs", instance.WardenJobId)
}

func (instance *Instance) StopListening() {
	instance.listenerControlChannel <- true
}

func (instance *Instance) StartListening(loggregatorClient LoggregatorClient) {
	instance.listenerControlChannel = make(chan bool)
	stdoutSocket := filepath.Join(instance.Identifier(), "stdout.sock")
	go instance.listen(stdoutSocket, loggregatorClient)
}

func (instance *Instance) listen(socket string, loggregatorClient LoggregatorClient) {
	connection, error := net.Dial("unix", socket)
	if error != nil {
		logger.Fatalf("Error while dialing into socket %s, %e", socket, error)
		panic(error)
	}
	buffer := make([]byte, 128)

	for {
		readCount, error := connection.Read(buffer)
		if error != nil {
			logger.Warnf("Error while reading from socket %s, %e", socket, error)
			break
		}
		logger.Debugf("Read %i bytes from socket", readCount)
		loggregatorClient.Send(buffer[:readCount])
		logger.Debugf("Sent %i bytes to loggregator", readCount)
		runtime.Gosched()
		select {
		case stop := <-instance.listenerControlChannel:
			if stop {
				logger.Debugf("Stopped listening to instance %v", instance.Identifier())
				close(instance.listenerControlChannel)
				break
			}
		}
	}
}
