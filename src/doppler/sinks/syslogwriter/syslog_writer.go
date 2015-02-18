//
// Forked and simplified from http://golang.org/src/pkg/log/syslog/syslog.go
// Fork needed to set the proper hostname in the write() function
//

package syslogwriter

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
)

type syslogWriter struct {
	appId     string
	raddr     string
	scheme    string
	outputUrl *url.URL
	connected bool

	mu   sync.Mutex // guards conn
	conn net.Conn
}

func NewSyslogWriter(outputUrl *url.URL, appId string) (w *syslogWriter, err error) {
	if outputUrl.Scheme != "syslog" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, syslogWriter only supports syslog", outputUrl.Scheme))
	}
	return &syslogWriter{
		appId:     appId,
		outputUrl: outputUrl,
		raddr:     outputUrl.Host,
		connected: false,
		scheme:    outputUrl.Scheme,
	}, nil
}

func (w *syslogWriter) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.connect()
	return err
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *syslogWriter) connect() error {
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.Close()
		w.conn = nil
	}
	c, err := net.Dial("tcp", w.raddr)
	if err == nil {
		w.conn = c
	}
	return err
}

func (w *syslogWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (int, error) {
	return w.write(p, source, sourceId, string(b), timestamp)
}

func (w *syslogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		return err
	}
	return nil
}

func (w *syslogWriter) write(p int, source string, sourceId string, msg string, timestamp int64) (byteCount int, err error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, msg, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := []byte(fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg))

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		byteCount, err = w.conn.Write(finalMsg)
	}
	return byteCount, err
}
