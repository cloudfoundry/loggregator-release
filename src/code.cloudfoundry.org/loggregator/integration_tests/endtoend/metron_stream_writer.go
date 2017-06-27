package endtoend

import (
	"fmt"
	"net"
)

type MetronStreamWriter struct {
	metronConn net.Conn
	Writes     int
}

func NewMetronStreamWriter(metronPort int) *MetronStreamWriter {
	metronConn, err := net.Dial("udp4", fmt.Sprintf("127.0.0.1:%d", metronPort))
	if err != nil {
		panic(err)
	}
	return &MetronStreamWriter{metronConn: metronConn}
}

func (w *MetronStreamWriter) Write(b []byte) {
	w.Writes++
	_, err := w.metronConn.Write(b)
	if err != nil {
		panic(err)
	}
}
