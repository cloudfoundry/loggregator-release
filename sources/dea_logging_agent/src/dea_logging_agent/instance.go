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
		panic(error)
	}
	buffer := make([]byte, 128)

	for {
		readCount, error := connection.Read(buffer)
		if error != nil {
			break
		}
		loggregatorClient.Send(buffer[:readCount])
		runtime.Gosched()
		select {
		case stop := <-instance.listenerControlChannel:
			if stop {
				close(instance.listenerControlChannel)
				break
			}
		}
	}
}
