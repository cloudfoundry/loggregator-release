package deaagent

import (
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"deaagent/loggregatorclient"
	"github.com/cloudfoundry/gosteno"
)

type instance struct {
	applicationId                  string
	wardenJobId                    uint64
	wardenContainerPath            string
	index                          uint64
	logger 	                       *gosteno.Logger
}

func (instance *instance) identifier() string {
	return filepath.Join(instance.wardenContainerPath, "jobs", strconv.FormatUint(instance.wardenJobId, 10))
}

func (inst *instance) startListening(loggregatorClient loggregatorclient.LoggregatorClient) {
	logPrefix := func(inst *instance, socketName string) (string) {
		return strings.Join([]string{inst.applicationId, strconv.FormatUint(inst.index, 10), socketName}, " ")
	}

	listen := func(logger *gosteno.Logger, socket string, loggregatorClient loggregatorclient.LoggregatorClient, prefix string) {
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
			logger.Debugf("Read %d bytes from instance socket", readCount)
			loggregatorClient.Send(buffer[:readCount + prefixLength])
			logger.Debugf("Sent %d bytes to loggregator client", readCount)
			runtime.Gosched()
		}
	}

	stdoutSocket := filepath.Join(inst.identifier(), "stdout.sock")
	go listen(inst.logger, stdoutSocket, loggregatorClient, logPrefix(inst, "STDOUT"))

	stderrSocket := filepath.Join(inst.identifier(), "stderr.sock")
	go listen(inst.logger, stderrSocket, loggregatorClient, logPrefix(inst, "STDERR"))
}
