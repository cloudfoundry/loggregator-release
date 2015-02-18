//
// Forked and simplified from http://golang.org/src/pkg/log/syslog/syslog.go
// Fork needed to set the proper hostname in the write() function
//

package syslogwriter

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type httpWriter struct {
	appId     string
	raddr     string
	scheme    string
	outputUrl *url.URL

	mu sync.Mutex // guards conn

	tlsConfig *tls.Config
	client    *http.Client
}

func NewHttpWriter(outputUrl *url.URL, appId string, skipCertVerify bool) (w *httpWriter, err error) {
	if outputUrl.Scheme != "https" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, httpWriter only supports https", outputUrl.Scheme))
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: skipCertVerify}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: tr}
	return &httpWriter{
		appId:     appId,
		outputUrl: outputUrl,
		raddr:     outputUrl.Host,
		scheme:    outputUrl.Scheme,
		tlsConfig: tlsConfig,
		client:    client,
	}, nil
}

func (w *httpWriter) Connect() error {
	return nil
}

func (w *httpWriter) WriteStdout(b []byte, source string, sourceId string, timestamp int64) (int, error) {
	return w.write(14, source, sourceId, string(b), timestamp)
}

func (w *httpWriter) WriteStderr(b []byte, source string, sourceId string, timestamp int64) (int, error) {
	return w.write(11, source, sourceId, string(b), timestamp)
}

func (w *httpWriter) Close() error {
	return nil
}

func (w *httpWriter) write(p int, source string, sourceId string, msg string, timestamp int64) (byteCount int, err error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, msg, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg)

	return w.writeHttp(finalMsg)
}

func (w *httpWriter) writeHttp(finalMsg string) (byteCount int, err error) {
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

func (w *httpWriter) IsConnected() bool {
	return true
}

func (w *httpWriter) SetConnected(bool) {
}
