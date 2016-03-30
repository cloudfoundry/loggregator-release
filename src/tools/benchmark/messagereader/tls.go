package messagereader

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
)

type TLSReader struct {
	listener  net.Listener
	config    *tls.Config
	envelopes chan *events.Envelope
}

func NewTLS(port int, config *tls.Config) *TLSReader {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	reader := &TLSReader{
		listener:  tcpListener,
		config:    config,
		envelopes: make(chan *events.Envelope, 10),
	}
	go reader.listen()
	return reader
}

func (r *TLSReader) listen() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			panic(err)
		}
		go r.readFromConn(conn)
	}
}

func (r *TLSReader) readFromConn(conn net.Conn) {
	tlsConn := tls.Server(conn, r.config)
	buffer := make([]byte, 1024)
	var size uint32
	for {
		err := binary.Read(tlsConn, binary.LittleEndian, &size)
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		if len(buffer) < int(size) {
			buffer = make([]byte, size)
		}
		count, err := io.ReadFull(tlsConn, buffer[:size])
		if err != nil {
			panic(err)
		}
		r.addMessage(buffer[:count])
	}
}

func (r *TLSReader) addMessage(bytes []byte) {
	env := &events.Envelope{}
	if err := env.Unmarshal(bytes); err != nil {
		panic(err)
	}
	r.envelopes <- env
}

func (r *TLSReader) Read() *events.Envelope {
	return <-r.envelopes
}

func (r *TLSReader) Close() {
}
