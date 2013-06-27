package dea_logging_agent

import (
	"net"
	"path/filepath"
//	"runtime"
)

type Instance struct {
	ApplicationId                  string
	WardenJobId                    string
	WardenContainerPath            string
}

func (instance *Instance) Identifier() string {
	return filepath.Join(instance.WardenContainerPath, "jobs", instance.WardenJobId)
}

func (instance *Instance) StartListening(loggregatorClient LoggregatorClient) {
	stdoutSocket := filepath.Join(instance.Identifier(), "stdout.sock")
	go instance.listen(stdoutSocket, loggregatorClient, "STDOUT ")

	stderrSocket := filepath.Join(instance.Identifier(), "stderr.sock")
	go instance.listen(stderrSocket, loggregatorClient, "STDERR ")
}

func (instance *Instance) listen(socket string, loggregatorClient LoggregatorClient, prefix string) {
	connection, err := net.Dial("unix", socket)
	if err != nil {
		logger.Fatalf("Error while dialing into socket %s, %s", socket, err)
		return
	}
//	defer connection.Close()
//		logger.Infof("Stopped reading from socket %s", socket)
//	}()
	prefixBytes := []byte(prefix)
	prefixLength := len(prefixBytes)
	buffer := make([]byte, prefixLength + bufferSize)

	//we're copying (and keeping) the message prefix in the buffer so every loggregatorClient.Send will have the prefix
	copy(buffer[0:prefixLength], prefixBytes)

	for {
		readCount, error := connection.Read(buffer[prefixLength:])
		if readCount == 0 && error != nil {
			logger.Infof("Error while reading from socket %s, %s", socket, error)
			break
		}
		logger.Debugf("Read %d bytes from socket", readCount)
		loggregatorClient.Send(buffer[:readCount + prefixLength])
		logger.Debugf("Sent %d bytes to loggregator", readCount)
//		runtime.Gosched()
	}
}

