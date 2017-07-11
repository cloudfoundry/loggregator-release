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

	"code.cloudfoundry.org/loggregator/plumbing"
)

type tlsWriter struct {
	appId    string
	host     string
	hostname string

	mu        sync.Mutex // guards conn
	conn      net.Conn
	dialer    *net.Dialer
	ioTimeout time.Duration

	TlsConfig *tls.Config
}

func NewTlsWriter(outputUrl *url.URL, appId, hostname string, skipCertVerify bool, dialer *net.Dialer, ioTimeout time.Duration) (w *tlsWriter, err error) {
	if dialer == nil {
		return nil, errors.New("cannot construct a writer with a nil dialer")
	}

	if outputUrl.Scheme != "syslog-tls" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, tlsWriter only supports syslog-tls", outputUrl.Scheme))
	}

	tlsConfig := plumbing.NewTLSConfig()
	tlsConfig.InsecureSkipVerify = skipCertVerify
	return &tlsWriter{
		appId:     appId,
		hostname:  hostname,
		host:      outputUrl.Host,
		TlsConfig: tlsConfig,
		dialer:    dialer,
		ioTimeout: ioTimeout,
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
	c, err := tls.DialWithDialer(w.dialer, "tcp", w.host, w.TlsConfig)
	if err == nil {
		w.conn = c
	}
	return err
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

func (w *tlsWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (byteCount int, err error) {
	syslogMsg := createMessage(p, w.appId, w.hostname, source, sourceId, b, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := []byte(fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg))

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn == nil {
		return 0, errors.New("Connection to syslog-tls sink lost")
	}

	if w.ioTimeout != 0 {
		// set both timeouts to catch handshake reads
		w.conn.SetDeadline(time.Now().Add(w.ioTimeout))
	}

	return w.conn.Write(finalMsg)
}
