package endtoend

import (
	"net"
)

type MetronStreamWriter struct {
	metronConn net.Conn
	Writes     int
}

func NewMetronStreamWriter() *MetronStreamWriter {
	metronConn, err := net.Dial("udp4", "127.0.0.1:49625")
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
