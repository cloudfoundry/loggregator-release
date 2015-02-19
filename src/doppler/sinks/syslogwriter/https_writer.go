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

type httpsWriter struct {
	appId     string
	raddr     string
	scheme    string
	outputUrl *url.URL

	mu sync.Mutex // guards conn

	tlsConfig *tls.Config
	client    *http.Client
}

func NewHttpsWriter(outputUrl *url.URL, appId string, skipCertVerify bool) (w *httpsWriter, err error) {
	if outputUrl.Scheme != "https" {
		return nil, errors.New(fmt.Sprintf("Invalid scheme %s, httpsWriter only supports https", outputUrl.Scheme))
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: skipCertVerify}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: tr}
	return &httpsWriter{
		appId:     appId,
		outputUrl: outputUrl,
		raddr:     outputUrl.Host,
		scheme:    outputUrl.Scheme,
		tlsConfig: tlsConfig,
		client:    client,
	}, nil
}

func (w *httpsWriter) Connect() error {
	return nil
}

func (w *httpsWriter) Write(p int, b []byte, source string, sourceId string, timestamp int64) (int, error) {
	syslogMsg := createMessage(p, w.appId, source, sourceId, b, timestamp)
	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	finalMsg := fmt.Sprintf("%d %s", len(syslogMsg), syslogMsg)

	return w.writeHttp(finalMsg)
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
