package accesslogger

import (
	"io"
	"net/http"
	"time"

	"github.com/cloudfoundry/gosteno"
)

//go:generate hel --type Writer --output mock_writer_test.go

type Writer interface {
	Write(message []byte) (sent int, err error)
}

type AccessLogger struct {
	writer io.Writer
	logger *gosteno.Logger
}

func New(writer io.Writer, logger *gosteno.Logger) *AccessLogger {
	return &AccessLogger{
		writer: writer,
		logger: logger,
	}
}

func (a *AccessLogger) LogAccess(req *http.Request, host string, port uint32) error {
	al := NewAccessLog(req, time.Now(), host, port, a.logger)
	_, err := a.writer.Write([]byte(al.String() + "\n"))
	return err
}
