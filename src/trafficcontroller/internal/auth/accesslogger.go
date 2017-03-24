package auth

import (
	"io"
	"net/http"
	"time"
)

//go:generate hel --type Writer --output mock_writer_test.go

type Writer interface {
	Write(message []byte) (sent int, err error)
}

type DefaultAccessLogger struct {
	writer io.Writer
}

func NewAccessLogger(writer io.Writer) *DefaultAccessLogger {
	return &DefaultAccessLogger{
		writer: writer,
	}
}

func (a *DefaultAccessLogger) LogAccess(req *http.Request, host string, port uint32) error {
	al := NewAccessLog(req, time.Now(), host, port)
	_, err := a.writer.Write([]byte(al.String() + "\n"))
	return err
}
