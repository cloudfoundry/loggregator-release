package metrics

import (
	"encoding/binary"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type TCPTestDoppler struct {
	listener     net.Listener
	readMessages uint32
	MessageChan  chan *events.Envelope
}

func NewTCPTestDoppler(address string) *TCPTestDoppler {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	return &TCPTestDoppler{
		listener:    listener,
		MessageChan: make(chan *events.Envelope),
	}
}

func (d *TCPTestDoppler) Start() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			return
		}
		go d.readFromConn(conn)
	}
}

func (d *TCPTestDoppler) Stop() {
	d.listener.Close()
}

func (d *TCPTestDoppler) readFromConn(conn net.Conn) {
	for {
		var size uint32
		err := binary.Read(conn, binary.LittleEndian, &size)
		if err != nil {
			return
		}

		buff := make([]byte, size)
		readCount, err := conn.Read(buff)
		if err != nil || readCount != int(size) {
			return
		}
		var env events.Envelope
		if err := proto.Unmarshal(buff, &env); err != nil {
			return
		}
		d.MessageChan <- &env
	}
}
