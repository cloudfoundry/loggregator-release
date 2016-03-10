package statsdemitter

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type StatsdEmitter struct {
	port   uint
	logger *gosteno.Logger
}

func New(port uint, logger *gosteno.Logger) *StatsdEmitter {
	return &StatsdEmitter{
		port:   port,
		logger: logger,
	}
}

func (s *StatsdEmitter) Run(inputChan chan *events.Envelope) {
	conn, err := net.Dial("udp4", fmt.Sprintf(":%d", s.port))
	if err != nil {
		panic(err)
	}
	for {
		message, ok := <-inputChan
		if !ok {
			conn.Close()
			return
		}
		bytes, err := proto.Marshal(message)
		if err != nil {
			s.logger.Errorf("Error while marshaling envelope: %s", err.Error())
			continue
		}
		s.logger.Debug("Sending envelope to metron")
		bytesWritten, err := conn.Write(bytes)
		if err != nil {
			s.logger.Errorf("Error writing to metron: %v\n", err)
		}
		s.logger.Debugf("Written %d bytes\n", bytesWritten)
	}
}
