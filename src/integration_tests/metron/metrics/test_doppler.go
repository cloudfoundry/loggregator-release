package metrics

import (
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type TestDoppler struct {
	conn        net.PacketConn
	stopChan    chan struct{}
	MessageChan chan *events.Envelope
}

func NewTestDoppler() *TestDoppler {
	c, err := net.ListenPacket("udp", "localhost:3457")
	if err != nil {
		panic(err)
	}
	return &TestDoppler{
		conn:        c,
		stopChan:    make(chan struct{}),
		MessageChan: make(chan *events.Envelope),
	}
}

func (d *TestDoppler) Start() {
	readBuffer := make([]byte, 65535)

	for {
		readCount, _, _ := d.conn.ReadFrom(readBuffer)
		if readCount > 0 {
			readData := make([]byte, readCount)
			copy(readData, readBuffer[:readCount])

			var envelope events.Envelope
			err := proto.Unmarshal(readData[32:], &envelope)

			if err != nil {
				panic(err)
			}

			d.MessageChan <- &envelope

			select {
			case <-d.stopChan:
				return
			default:
			}
		}
	}
}

func (d *TestDoppler) Stop() {
	close(d.stopChan)
	d.conn.Close()
}
