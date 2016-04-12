package accesslogger

import (
	"fmt"
	"io"
	"net/http"
)

//go:generate hel --type Writer --output mock_writer_test.go

type Writer interface {
	Write(message []byte) (sent int, err error)
}

type AccessLogger struct {
	writer io.Writer
}

func New(writer io.Writer) *AccessLogger {
	return &AccessLogger{writer: writer}
}

func (a *AccessLogger) LogAccess(req *http.Request) error {
	remoteAddr := req.Header.Get("X-Forwarded-For")
	if remoteAddr == "" {
		remoteAddr = req.RemoteAddr
	}
	scheme := "http"
	if req.TLS != nil {
		scheme += "s"
	}
	requestSource := fmt.Sprintf("%s %s: %s://%s%s\n", remoteAddr, req.Method, scheme, req.Host, req.URL.Path)
	_, err := a.writer.Write([]byte(requestSource))
	return err
}
