package deaagent

import (
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"deaagent/loggregatorclient"
)

type instance struct {
	applicationId                  string
	wardenJobId                    uint64
	wardenContainerPath            string
	index                          uint64
}

func (instance *instance) identifier() string {
	return filepath.Join(instance.wardenContainerPath, "jobs", strconv.FormatUint(instance.wardenJobId, 10))
}

func (instance *instance) startListening(loggregatorClient loggregatorclient.LoggregatorClient) {
	stdoutSocket := filepath.Join(instance.identifier(), "stdout.sock")
	go instance.listen(stdoutSocket, loggregatorClient, instance.logPrefix("STDOUT"))

	stderrSocket := filepath.Join(instance.identifier(), "stderr.sock")
	go instance.listen(stderrSocket, loggregatorClient, instance.logPrefix("STDERR"))
}

func (instance *instance) logPrefix(socketName string) (string) {
	return strings.Join([]string{instance.applicationId, strconv.FormatUint(instance.index, 10), socketName}, " ")
}

func (instance *instance) listen(socket string, loggregatorClient loggregatorclient.LoggregatorClient, prefix string) {
	connection, err := net.Dial("unix", socket)
	if err != nil {
		logger.Fatalf("Error while dialing into socket %s, %s", socket, err)
		return
	}
	defer func() {
		connection.Close()
		logger.Infof("Stopped reading from socket %s", socket)
	}()
	prefixBytes := []byte(prefix + " ")
	prefixLength := len(prefixBytes)
	buffer := make([]byte, prefixLength + bufferSize)

	//we're copying (and keeping) the message prefix in the buffer so every loggregatorClient.Send will have the prefix
	copy(buffer[0:prefixLength], prefixBytes)

	for {
		readCount, err := connection.Read(buffer[prefixLength:])
		if readCount == 0 && err != nil {
			logger.Infof("Error while reading from socket %s, %s", socket, err)
			break
		}
		logger.Debugf("Read %d bytes from socket", readCount)
		loggregatorClient.Send(buffer[:readCount + prefixLength])
		logger.Debugf("Sent %d bytes to loggregator", readCount)
		runtime.Gosched()
	}
}
