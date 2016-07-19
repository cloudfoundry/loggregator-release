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

func NewTestDoppler(address string) *TestDoppler {
	c, err := net.ListenPacket("udp4", address)
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
		readCount, _, err := d.conn.ReadFrom(readBuffer)
		if err != nil {
			return
		}

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
