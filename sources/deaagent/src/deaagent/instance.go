package deaagent

import (
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Instance struct {
	ApplicationId                  string
	WardenJobId                    uint64
	WardenContainerPath            string
	Index                          uint64
}

func (instance *Instance) Identifier() string {
	return filepath.Join(instance.WardenContainerPath, "jobs", strconv.FormatUint(instance.WardenJobId, 10))
}

func (instance *Instance) StartListening(loggregatorClient LoggregatorClient) {
	stdoutSocket := filepath.Join(instance.Identifier(), "stdout.sock")
	go instance.listen(stdoutSocket, loggregatorClient, instance.logPrefix("STDOUT"))

	stderrSocket := filepath.Join(instance.Identifier(), "stderr.sock")
	go instance.listen(stderrSocket, loggregatorClient, instance.logPrefix("STDERR"))
}

func (instance *Instance) logPrefix(socketName string) (string) {
	return strings.Join([]string{instance.ApplicationId, strconv.FormatUint(instance.Index, 10), socketName}, " ")
}

func (instance *Instance) listen(socket string, loggregatorClient LoggregatorClient, prefix string) {
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
