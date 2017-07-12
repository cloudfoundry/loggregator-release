//
// Forked and simplified from http://golang.org/src/pkg/log/syslog/syslog.go
// Fork needed to set the proper hostname in the write() function
//

package syslogwriter

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"
)

type httpsWriter struct {
	appId     string
	hostname  string
	outputUrl *url.URL

	mu sync.Mutex // guards lastError

	TlsConfig *tls.Config
	client    *http.Client
	lastError error
}

func NewHttpsWriter(outputUrl *url.URL, appId, hostname string, skipCertVerify bool, dialer *net.Dialer, timeout time.Duration) (w *httpsWriter, err error) {
	if dialer == nil {
		return nil, errors.New("cannot construct a writer with a nil dialer")
	}

	if outputUrl.Scheme != "https" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, httpsWriter only supports https", outputUrl.Scheme))
	}

	tlsConfig := plumbing.NewTLSConfig()
	tlsConfig.InsecureSkipVerify = skipCertVerify
	tr := &http.Transport{
		MaxIdleConnsPerHost: 1,
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: dialer.Timeout * 2,
		Dial: func(network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	client := &http.Client{Transport: tr, Timeout: timeout}
	return &httpsWriter{
		appId:     appId,
		hostname:  hostname,
		outputUrl: outputUrl,
		TlsConfig: tlsConfig,
		client:    client,
	}, nil
}

func (w *httpsWriter) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lastError != nil {
		err := w.lastError
		w.lastError = nil
		return err
	}
	return nil
}

func (w *httpsWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (int, error) {
	syslogMsg := createMessage(p, w.appId, w.hostname, source, sourceId, b, timestamp)
	bytesWritten, err := w.writeHttp(syslogMsg)
	w.mu.Lock()
	w.lastError = err
	w.mu.Unlock()
	return bytesWritten, err
}

func (w *httpsWriter) Close() error {
	return nil
}

func (w *httpsWriter) writeHttp(finalMsg string) (byteCount int, err error) {
	resp, err := w.client.Post(w.outputUrl.String(), "text/plain", strings.NewReader(finalMsg))
	byteCount = len(finalMsg)
	if err != nil {
		return byteCount, errors.New("syslog https writer: failed to connect")
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err = fmt.Errorf("Syslog Writer: Post responded with %d status code", resp.StatusCode)
	}
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	return byteCount, err
}
