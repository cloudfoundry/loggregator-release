//
// Forked and simplified from http://golang.org/src/pkg/log/syslog/syslog.go
// Fork needed to set the proper hostname in the write() function
//

package syslogwriter

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"
)

type tlsWriter struct {
	appId string
	host  string

	mu   sync.Mutex // guards conn
	conn net.Conn

	tlsConfig *tls.Config
}

func NewTlsWriter(outputUrl *url.URL, appId string, skipCertVerify bool) (w *tlsWriter, err error) {
	if outputUrl.Scheme != "syslog-tls" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, tlsWriter only supports syslog-tls", outputUrl.Scheme))
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: skipCertVerify}
	return &tlsWriter{
		appId:     appId,
		host:      outputUrl.Host,
		tlsConfig: tlsConfig,
	}, nil
}

func (w *tlsWriter) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.Close()
		w.conn = nil
	}
	dialer := new(net.Dialer)
	dialer.Timeout = 500 * time.Millisecond
	c, err := tls.DialWithDialer(dialer, "tcp", w.host, w.tlsConfig)
	if err == nil {
		w.conn = c
	}
	return err
}

func (w *tlsWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (byteCount int, err error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, b, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := []byte(fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg))

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		byteCount, err = w.conn.Write(finalMsg)
	} else {
		return 0, errors.New("Connection to syslog-tls sink lost")
	}
	return byteCount, err
}

func (w *tlsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		return err
	}
	return nil
}
