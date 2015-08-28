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
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type httpsWriter struct {
	appId     string
	outputUrl *url.URL

	mu sync.Mutex // guards conn

	tlsConfig *tls.Config
	client    *http.Client
	lastError error
}

func NewHttpsWriter(outputUrl *url.URL, appId string, skipCertVerify bool, dialer *net.Dialer, timeout time.Duration) (w *httpsWriter, err error) {
	if dialer == nil {
		return nil, errors.New("cannot construct a writer with a nil dialer")
	}

	if outputUrl.Scheme != "https" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, httpsWriter only supports https", outputUrl.Scheme))
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: skipCertVerify}
	tr := &http.Transport{
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: dialer.Timeout * 2,
		Dial: func(network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	return &httpsWriter{
		appId:     appId,
		outputUrl: outputUrl,
		tlsConfig: tlsConfig,
		client:    client,
	}, nil
}

func (w *httpsWriter) Connect() error {
	if w.lastError != nil {
		err := w.lastError
		w.lastError = nil
		return err
	}
	return nil
}

func (w *httpsWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (int, error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, b, timestamp)
	var bytesWritten int
	bytesWritten, w.lastError = w.writeHttp(syslogMsg)
	return bytesWritten, w.lastError
}

func (w *httpsWriter) Close() error {
	return nil
}

func (w *httpsWriter) writeHttp(finalMsg string) (byteCount int, err error) {
	resp, err := w.client.Post(w.outputUrl.String(), "text/plain", strings.NewReader(finalMsg))
	if resp != nil {
		if resp.StatusCode != 200 {
			err = errors.New("Syslog Writer: Post responded with a non 200 status code")
		}
		resp.Body.Close()
	}
	byteCount = len(finalMsg)
	return byteCount, err
}
