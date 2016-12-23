package statsdemitter

import (
	"fmt"
	"log"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type StatsdEmitter struct {
	port uint
}

func New(port uint) *StatsdEmitter {
	return &StatsdEmitter{
		port: port,
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
			log.Printf("Error while marshaling envelope: %s", err)
			continue
		}
		_, err = conn.Write(bytes)
		if err != nil {
			log.Printf("Error writing to metron: %v\n", err)
		}
	}
}
