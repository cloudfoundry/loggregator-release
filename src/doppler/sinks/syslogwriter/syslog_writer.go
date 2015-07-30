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
	"time"
)

type syslogWriter struct {
	appId string
	host  string

	mu   sync.Mutex // guards conn
	conn net.Conn
}

func NewSyslogWriter(outputUrl *url.URL, appId string) (w *syslogWriter, err error) {
	if outputUrl.Scheme != "syslog" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, syslogWriter only supports syslog", outputUrl.Scheme))
	}
	return &syslogWriter{
		appId: appId,
		host:  outputUrl.Host,
	}, nil
}

func (w *syslogWriter) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.Close()
		w.conn = nil
	}

	c, err := net.DialTimeout("tcp", w.host, 500*time.Millisecond)
	if err != nil {
		return err
	}

	w.conn = c
	w.watchConnection()

	return nil
}

func (w *syslogWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (byteCount int, err error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, b, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := []byte(fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg))

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn == nil {
		return 0, errors.New("Connection to syslog sink lost")
	}

	return w.conn.Write(finalMsg)
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

func (w *syslogWriter) watchConnection() {
	conn := w.conn

	go func() {
		buffer := make([]byte, 1)
		for {
			_, err := conn.Read(buffer)
			if err != nil {
				w.Close()
				return
			}
		}
	}()
}
