package plumbing

import (
	"io"
	"os"
	"time"
)

type LogWriter struct {
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	str := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") + " " + string(bytes)
	return io.WriteString(os.Stderr, str)
}
