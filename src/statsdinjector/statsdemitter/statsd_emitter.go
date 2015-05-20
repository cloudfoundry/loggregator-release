package statsdemitter

import (
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/coreos/etcd/third_party/code.google.com/p/goprotobuf/proto"
	"net"
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
	conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", s.port))
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
		conn.Write(bytes)
	}
}
