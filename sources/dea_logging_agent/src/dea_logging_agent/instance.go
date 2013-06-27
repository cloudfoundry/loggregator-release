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
	go instance.listen(stdoutSocket, loggregatorClient, "STDOUT ")

	stderrSocket := filepath.Join(instance.Identifier(), "stderr.sock")
	go instance.listen(stderrSocket, loggregatorClient, "STDERR ")
}

func (instance *Instance) listen(socket string, loggregatorClient LoggregatorClient, prefix string) {
	connection, error := net.Dial("unix", socket)
	defer connection.Close()
	if error != nil {
		logger.Fatalf("Error while dialing into socket %s, %e", socket, error)
		panic(error)
	}

	prefixBytes := []byte(prefix)
	prefixLength := len(prefixBytes)
	buffer := make([]byte, prefixLength + bufferSize)

	//we're copying (and keeping) the message prefix in the buffer so every loggregatorClient.Send will have the prefix
	copy(buffer[0:prefixLength], prefixBytes)

	for {
		readCount, error := connection.Read(buffer[prefixLength:])
		if error != nil {
			logger.Warnf("Error while reading from socket %s, %e", socket, error)
			break
		}
		logger.Debugf("Read %d bytes from socket", readCount)
		loggregatorClient.Send(buffer[:readCount + prefixLength])
		logger.Debugf("Sent %d bytes to loggregator", readCount)
		runtime.Gosched()
		select {
		case stop := <-instance.listenerControlChannel:
			if stop {
				logger.Debugf("Stopped listening to instance %v", instance.Identifier())
				close(instance.listenerControlChannel)
				break
			}
		default:
			// Don't Block... continue
		}
	}
}
